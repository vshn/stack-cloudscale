/*
Copyright (c) 2019, VSHN AG, info@vshn.ch

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

package s3

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	storagev1alpha1 "github.com/vshn/stack-cloudscale/api/storage/v1alpha1"
	cloudscalev1alpha1 "github.com/vshn/stack-cloudscale/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/vshn/stack-cloudscale/clients/s3"
)

const (
	errNotInstance = "managed resource is not an S3Bucket"

	statusOnline   = "Online"
	statusCreating = "Creating"
	statusDeleting = "Deleting"

	resourceCredentialsSecretBucketname = "bucketname"
)

var log = logging.Logger.WithName("s3bucket_controller")

// BucketController is responsible for adding the S3Bucket
// controller and its corresponding reconciler to the manager with any runtime configuration.
type BucketController struct{}

// SetupWithManager instantiates a new controller using a resource.ManagedReconciler
// configured to reconcile S3Buckets using an ExternalClient produced by
// connecter, which satisfies the ExternalConnecter interface.
func (r *BucketController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(strings.ToLower(storagev1alpha1.S3BucketKindAPIVersion)).
		For(&storagev1alpha1.S3Bucket{}).
		Owns(&corev1.Secret{}).
		Complete(resource.NewManagedReconciler(mgr,
			resource.ManagedKind(storagev1alpha1.S3BucketGroupVersionKind),
			resource.WithExternalConnecter(&connecter{client: mgr.GetClient(), newS3Client: s3.NewClient})))
}

// Connecter satisfies the resource.ExternalConnecter interface.
type connecter struct {
	client      client.Client
	newS3Client func(ctx context.Context, cloudscaleToken string, httpClient *http.Client) s3.Service
}

// Connect to the supplied resource.Managed (presumed to be a
// S3Bucket) by using the Provider it references to create a new
// S3 client.
func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	// Assert that resource.Managed we were passed in fact contains an
	// S3Bucket. We told NewControllerManagedBy that this was a
	// controller For S3Bucket, so something would have to go
	// horribly wrong for us to encounter another type.
	i, ok := mg.(*storagev1alpha1.S3Bucket)
	if !ok {
		return nil, errors.New(errNotInstance)
	}

	// Get the Provider referenced by the S3Bucket.
	p := &cloudscalev1alpha1.Provider{}
	if err := c.client.Get(ctx, meta.NamespacedNameOf(i.Spec.ProviderReference), p); err != nil {
		return nil, errors.Wrap(err, "cannot get Provider")
	}

	// Get the Secret referenced by the Provider.
	s := &corev1.Secret{}
	n := types.NamespacedName{Namespace: p.Spec.Secret.Namespace, Name: p.Spec.Secret.Name}
	if err := c.client.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get Provider secret %s", n)
	}

	// Create and return a new S3 client using the credentials read from
	// our Provider's Secret.
	client := c.newS3Client(ctx, string(s.Data[p.Spec.Secret.Key]), nil)
	ext := &external{
		s3Client: client,
	}
	return ext, nil
}

type external struct {
	s3Client s3.Service
}

// Observe the existing external resource, if any. The resource.ManagedReconciler
// calls Observe in order to determine whether an external resource needs to be
// created, updated, or deleted.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	bucket, ok := mg.(*storagev1alpha1.S3Bucket)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotInstance)
	}
	log.Info("Observe", "bucket", bucket.Name)

	bucketName := meta.GetExternalName(bucket)

	bucketUser, err := e.s3Client.GetBucketInfo(ctx, bucket.Status.AtProvider.ObjectUserID, bucketName, bucket.Spec.ForProvider.Region)

	// If we encounter an error indicating the external resource does not exist
	// we want to let the resource.ManagedReconciler know so it can create it.
	if s3.IsErrorNotFound(err) {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, "cannot get bucket")
	}

	// Update our "Ready" status condition to reflect the status of the external
	// resource. Most managed resources use the below well known reasons that
	// the "Ready" status may be true or false, but managed resource authors
	// are welcome to define and use their own.
	switch bucket.Status.Status {
	case statusOnline:
		// If the resource is available we also want to mark it as bindable to
		// resource claims.
		bucket.SetConditions(runtimev1alpha1.Available())
		resource.SetBindable(bucket)
	case statusCreating:
		bucket.SetConditions(runtimev1alpha1.Creating())
	case statusDeleting:
		bucket.SetConditions(runtimev1alpha1.Deleting())
	}

	accessKey, secretKey, err := s3.GetKeys(bucketUser)
	if err != nil {
		return resource.ExternalObservation{}, err
	}
	bucket.Status.AtProvider.ObjectUserID = bucketUser.ID
	bucket.Status.Status = statusOnline

	// Finally, we report what we know about the external resource. Any
	// ConnectionDetails we return will be published to the managed resource's
	// connection secret if it specified one.
	o := resource.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
		ConnectionDetails: resource.ConnectionDetails{
			runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(accessKey),
			runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(secretKey),
			runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(fmt.Sprintf(s3.S3EndpointFormat, bucket.Spec.ForProvider.Region)),
			resourceCredentialsSecretBucketname:                  []byte(bucketName),
		},
	}

	return o, nil
}

// Create a new external resource based on the specification of our managed
// resource. resource.ManagedReconciler only calls Create if Observe reported
// that the external resource did not exist.
func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	bucket, ok := mg.(*storagev1alpha1.S3Bucket)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotInstance)
	}
	log.Info("Create", "bucket", bucket.Name)
	bucket.Status.Status = statusCreating

	bucketName := meta.GetExternalName(bucket)
	objectUser, err := e.s3Client.CreateOrUpdateBucket(ctx, bucket.Status.AtProvider.ObjectUserID, bucketName, bucket.Spec.ForProvider.Region, bucket.Spec.ForProvider.CannedACL, bucket.Spec.ForProvider.Tags)
	if err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, "cannot create bucket")
	}

	bucket.Status.AtProvider.ObjectUserID = objectUser.ID

	accessKey, secretKey, err := s3.GetKeys(objectUser)
	if err != nil {
		return resource.ExternalCreation{}, err
	}
	cn := resource.ConnectionDetails{
		runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(accessKey),
		runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(secretKey),
		runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(fmt.Sprintf(s3.S3EndpointFormat, bucket.Spec.ForProvider.Region)),
		resourceCredentialsSecretBucketname:                  []byte(bucketName),
	}

	return resource.ExternalCreation{ConnectionDetails: cn}, nil
}

// Update the existing external resource to match the specifications of our
// managed resource. resource.ManagedReconciler only calls Update if Observe
// reported that the external resource was not up to date.
func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	bucket, ok := mg.(*storagev1alpha1.S3Bucket)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotInstance)
	}
	log.Info("Update", "bucket", bucket.Name)
	objectUser, err := e.s3Client.CreateOrUpdateBucket(ctx, bucket.Status.AtProvider.ObjectUserID, meta.GetExternalName(bucket), bucket.Spec.ForProvider.Region, bucket.Spec.ForProvider.CannedACL, bucket.Spec.ForProvider.Tags)
	bucket.Status.AtProvider.ObjectUserID = objectUser.ID
	return resource.ExternalUpdate{}, errors.Wrap(err, "cannot update instance")
}

// Delete the external resource. resource.ManagedReconciler only calls Delete
// when a managed resource with the 'Delete' reclaim policy has been deleted.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	bucket, ok := mg.(*storagev1alpha1.S3Bucket)
	if !ok {
		return errors.New(errNotInstance)
	}
	log.Info("Delete", "bucket", bucket.Name)
	// Indicate that we're about to delete the instance.
	bucket.Status.Status = statusDeleting

	// Delete the instance.
	err := e.s3Client.DeleteBucket(ctx, bucket.Status.AtProvider.ObjectUserID, meta.GetExternalName(bucket), bucket.Spec.ForProvider.Region)
	if err != nil && !s3.IsErrorNotFound(err) {
		return errors.Wrap(err, "cannot delete instance")
	}
	return nil
}
