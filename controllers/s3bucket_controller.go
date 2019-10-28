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

package controllers

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudscalev1alpha1 "git.vshn.net/syn/stack-cloudscale/api/v1alpha1"
	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"git.vshn.net/syn/stack-cloudscale/clients/s3"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

const (
	errNotInstance = "managed resource is not an S3Bucket"

	statusOnline   = "Online"
	statusCreating = "Creating"
	statusDeleting = "Deleting"
)

// S3BucketInstanceController is responsible for adding the S3Bucket
// controller and its corresponding reconciler to the manager with any runtime configuration.
type S3BucketInstanceController struct{}

// SetupWithManager instantiates a new controller using a resource.ManagedReconciler
// configured to reconcile S3Buckets using an ExternalClient produced by
// connecter, which satisfies the ExternalConnecter interface.
func (r *S3BucketInstanceController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(strings.ToLower(cloudscalev1alpha1.S3BucketKindAPIVersion)).
		For(&cloudscalev1alpha1.S3Bucket{}).
		Owns(&corev1.Secret{}).
		Complete(resource.NewManagedReconciler(mgr,
			resource.ManagedKind(cloudscalev1alpha1.S3BucketGroupVersionKind),
			resource.WithExternalConnecter(&connecter{client: mgr.GetClient(), newS3Client: s3.NewClient})))
}

// Connecter satisfies the resource.ExternalConnecter interface.
type connecter struct {
	client      client.Client
	newS3Client func(ctx context.Context, credentials string) s3.Service
}

// Connect to the supplied resource.Managed (presumed to be a
// S3Bucket) by using the Provider it references to create a new
// S3 client.
func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	// Assert that resource.Managed we were passed in fact contains an
	// S3Bucket. We told NewControllerManagedBy that this was a
	// controller For S3Bucket, so something would have to go
	// horribly wrong for us to encounter another type.
	i, ok := mg.(*cloudscalev1alpha1.S3Bucket)
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
		return nil, errors.Wrapf(err, "cannot get Provider secret %v", n.String())
	}

	// Create and return a new S3 client using the credentials read from
	// our Provider's Secret.
	client := c.newS3Client(ctx, string(s.Data[p.Spec.Secret.Key]))
	ext := &external{
		s3Client:  client,
		k8sClient: c.client,
	}
	return ext, nil
}

type external struct {
	s3Client  s3.Service
	k8sClient client.Client
}

// Observe the existing external resource, if any. The resource.ManagedReconciler
// calls Observe in order to determine whether an external resource needs to be
// created, updated, or deleted.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	i, ok := mg.(*cloudscalev1alpha1.S3Bucket)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotInstance)
	}

	// Use our Cloudscale API client to get an up to date view of the external
	// resource.
	_, err := e.s3Client.GetBucketInfo(ctx, i)

	// If we encounter an error indicating the external resource does not exist
	// we want to let the resource.ManagedReconciler know so it can create it.
	if s3.IsErrorNotFound(err) {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}

	// Any other errors are wrapped (as is good Go practice) and returned to the
	// resource.ManagedReconciler. It will update the "Synced" status condition
	// of the managed resource to reflect that the most recent reconcile failed
	// and ensure the reconcile is reattempted after a brief wait.
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, "cannot get instance")
	}

	// The external resource exists. Copy any output-only fields to their
	// corresponding entries in our status field.
	//i.Status.Status = existing.Status.Status
	exists := true

	// Update our "Ready" status condition to reflect the status of the external
	// resource. Most managed resources use the below well known reasons that
	// the "Ready" status may be true or false, but managed resource authors
	// are welcome to define and use their own.
	switch i.Status.Status {
	case statusOnline:
		// If the resource is available we also want to mark it as bindable to
		// resource claims.
		i.SetConditions(runtimev1alpha1.Available())
		resource.SetBindable(i)
	case statusCreating:
		i.SetConditions(runtimev1alpha1.Creating())
		i.Status.Status = statusOnline
	case statusDeleting:
		i.SetConditions(runtimev1alpha1.Deleting())
		exists = false
	default:
		i.Status.Status = statusCreating
	}

	// Finally, we report what we know about the external resource. Any
	// ConnectionDetails we return will be published to the managed resource's
	// connection secret if it specified one.
	o := resource.ExternalObservation{
		ResourceExists:   exists,
		ResourceUpToDate: true,
		ConnectionDetails: resource.ConnectionDetails{
			runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("https://s3.cloudscale.ch"),
			"access_key": []byte("0ZTAIBKSGYBRHQ09G11W"),
			"secret_key": []byte("bn2ufcwbIa0ARLc5CLRSlVaCfFxPHOpHmjKiH34T"),
		},
	}

	return o, nil
}

// Create a new external resource based on the specification of our managed
// resource. resource.ManagedReconciler only calls Create if Observe reported
// that the external resource did not exist.
func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	i, ok := mg.(*cloudscalev1alpha1.S3Bucket)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotInstance)
	}

	// Create must return any connection details that are set or returned only
	// at creation time. The resource.ManagedReconciler will merge any details
	// with those returned during the Observe phase.
	cd := resource.ConnectionDetails{"secret_key": []byte("bn2ufcwbIa0ARLc5CLRSlVaCfFxPHOpHmjKiH34T")}

	// Create a new instance.
	err := e.s3Client.CreateOrUpdateBucket(ctx, i)
	if err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, "cannot create instance")
	}

	i.Status.Status = statusCreating

	return resource.ExternalCreation{ConnectionDetails: cd}, nil
}

// Update the existing external resource to match the specifications of our
// managed resource. resource.ManagedReconciler only calls Update if Observe
// reported that the external resource was not up to date.
func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	i, ok := mg.(*cloudscalev1alpha1.S3Bucket)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotInstance)
	}
	err := e.s3Client.CreateOrUpdateBucket(ctx, i)
	return resource.ExternalUpdate{}, errors.Wrap(err, "cannot update instance")
}

// Delete the external resource. resource.ManagedReconciler only calls Delete
// when a managed resource with the 'Delete' reclaim policy has been deleted.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	i, ok := mg.(*cloudscalev1alpha1.S3Bucket)
	if !ok {
		return errors.New(errNotInstance)
	}
	// Indicate that we're about to delete the instance.
	i.Status.Status = statusDeleting
	i.SetConditions(runtimev1alpha1.Deleting())

	// Delete the instance.
	err := e.s3Client.DeleteBucket(ctx, i)
	if err != nil {
		return errors.Wrap(err, "cannot delete instance")
	}

	// meta.RemoveFinalizer(i, "finalizer.managedresource.crossplane.io")
	// err = e.k8sClient.Update(ctx, i)

	return nil
}
