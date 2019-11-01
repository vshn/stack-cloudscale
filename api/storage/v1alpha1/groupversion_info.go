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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	// Group is the group of the objects
	Group = "storage.cloudscale.crossplane.io"

	// Version is the version of the objects
	Version = "v1alpha1"
)

var (

	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme

	// S3BucketKind is a convenience variable for the kind string
	S3BucketKind = reflect.TypeOf(S3Bucket{}).Name()

	// S3BucketKindAPIVersion is a convenience variable for the API version string
	S3BucketKindAPIVersion = S3BucketKind + "." + GroupVersion.String()

	// S3BucketGroupVersionKind is a convenience variable to generate the GroupVersionKind
	S3BucketGroupVersionKind = GroupVersion.WithKind(S3BucketKind)

	// S3BucketClassKind is a convenience variable for the kind string
	S3BucketClassKind = reflect.TypeOf(S3BucketClass{}).Name()

	// S3BucketClassKindAPIVersion is a convenience variable for the API version string
	S3BucketClassKindAPIVersion = S3BucketClassKind + "." + GroupVersion.String()

	// S3BucketClassGroupVersionKind is a convenience variable to generate the GroupVersionKind
	S3BucketClassGroupVersionKind = GroupVersion.WithKind(S3BucketClassKind)
)
