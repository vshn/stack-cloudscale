/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// S3BucketParameters define the desired state of a Cloudscale S3 Bucket
// https://www.cloudscale.ch/en/api/v1#objects-users
type S3BucketParameters struct {
	// NameFormat specifies the name of the external S3Bucket instance. The
	// first instance of the string '%s' will be replaced with the Kubernetes
	// UID of this S3Bucket. Omit this field to use the UID alone as the name.
	// +optional
	NameFormat string `json:"nameFormat,omitempty"`

	// Tags are optional key, value pairs to add to an S3 bucket
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

// S3BucketSpec defines the desired state of S3Bucket
type S3BucketSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	S3BucketParameters           `json:",inline"`
}

// S3BucketStatus defines the observed state of S3Bucket
type S3BucketStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`

	Status       string `json:"status,omitempty"`
	ObjectUserID string `json:"objectUserId,omitempty"`
	BucketName   string `json:"bucketName,omitempty"`
}

// +kubebuilder:object:root=true

// S3Bucket is the Schema for the s3buckets API
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type S3Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   S3BucketSpec   `json:"spec,omitempty"`
	Status S3BucketStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// S3BucketList contains a list of S3Bucket
type S3BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []S3Bucket `json:"items"`
}

// An S3BucketClassSpecTemplate is a template for the spec of a dynamically
// provisioned S3Bucket.
type S3BucketClassSpecTemplate struct {
	runtimev1alpha1.ClassSpecTemplate `json:",inline"`
	S3BucketParameters                `json:",inline"`
}

// +kubebuilder:object:root=true

// An S3BucketClass is a resource class. It defines the desired spec of resource
// claims that use it to dynamically provision a managed resource.
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".specTemplate.providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".specTemplate.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster
type S3BucketClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SpecTemplate is a template for the spec of a dynamically provisioned
	// S3Bucket.
	SpecTemplate S3BucketClassSpecTemplate `json:"specTemplate"`
}

// +kubebuilder:object:root=true

// S3BucketClassList contains a list of cloud memorystore resource classes.
type S3BucketClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []S3BucketClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&S3Bucket{}, &S3BucketList{})
	SchemeBuilder.Register(&S3BucketClass{}, &S3BucketClassList{})
}

// GetBucketName based on the NameFormat spec value,
// If name format is not provided, bucket name defaults to UID
// If name format provided with '%s' value, bucket name will result in formatted string + UID,
//   NOTE: only single %s substitution is supported
// If name format does not contain '%s' substitution, i.e. a constant string, the
// constant string value is returned back
//
// Examples:
//   For all examples assume "UID" = "test-uid"
//   1. NameFormat = "", BucketName = "test-uid"
//   2. NameFormat = "%s", BucketName = "test-uid"
//   3. NameFormat = "foo", BucketName = "foo"
//   4. NameFormat = "foo-%s", BucketName = "foo-test-uid"
//   5. NameFormat = "foo-%s-bar-%s", BucketName = "foo-test-uid-bar-%!s(MISSING)"
func (b *S3Bucket) GetBucketName() string {
	return util.ConditionalStringFormat(b.Spec.NameFormat, string(b.GetUID()))
}
