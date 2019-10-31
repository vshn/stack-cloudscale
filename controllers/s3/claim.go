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
	"strings"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	storagev1alpha1 "github.com/crossplaneio/crossplane/apis/storage/v1alpha1"
	cloudscalev1alpha1 "github.com/vshn/stack-cloudscale/api/v1alpha1"
)

// A BucketClaimSchedulingController reconciles Bucket claims that include a
// class selector but omit their class and resource references by picking a
// random matching S3BucketClass, if any.
type BucketClaimSchedulingController struct{}

// SetupWithManager sets up the BucketClaimSchedulingController using the
// supplied manager.
func (c *BucketClaimSchedulingController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("scheduler.%s.%s.%s",
		storagev1alpha1.BucketKind,
		cloudscalev1alpha1.S3BucketKind,
		cloudscalev1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&storagev1alpha1.Bucket{}).
		WithEventFilter(resource.NewPredicates(resource.AllOf(
			resource.HasClassSelector(),
			resource.HasNoClassReference(),
			resource.HasNoManagedResourceReference(),
		))).
		Complete(resource.NewClaimSchedulingReconciler(mgr,
			resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
			resource.ClassKind(cloudscalev1alpha1.S3BucketClassGroupVersionKind),
		))
}

// A BucketClaimDefaultingController reconciles Bucket claims that omit their
// resource ref, class ref, and class selector by choosing a default
// S3BucketClass if one exists.
type BucketClaimDefaultingController struct{}

// SetupWithManager sets up the BucketClaimDefaultingController using the
// supplied manager.
func (c *BucketClaimDefaultingController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("defaulter.%s.%s.%s",
		storagev1alpha1.BucketKind,
		cloudscalev1alpha1.S3BucketKind,
		cloudscalev1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&storagev1alpha1.Bucket{}).
		WithEventFilter(resource.NewPredicates(resource.AllOf(
			resource.HasNoClassSelector(),
			resource.HasNoClassReference(),
			resource.HasNoManagedResourceReference(),
		))).
		Complete(resource.NewClaimDefaultingReconciler(mgr,
			resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
			resource.ClassKind(cloudscalev1alpha1.S3BucketClassGroupVersionKind),
		))
}

// A BucketClaimController reconciles Bucket claims with S3Buckets, dynamically
// provisioning them if needed.
type BucketClaimController struct{}

// SetupWithManager adds a controller that reconciles Bucket resource claims.
func (c *BucketClaimController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		storagev1alpha1.BucketKind,
		cloudscalev1alpha1.S3BucketKind,
		cloudscalev1alpha1.Group))

	p := resource.NewPredicates(resource.AnyOf(
		resource.HasClassReferenceKind(resource.ClassKind(cloudscalev1alpha1.S3BucketClassGroupVersionKind)),
		resource.HasManagedResourceReferenceKind(resource.ManagedKind(cloudscalev1alpha1.S3BucketGroupVersionKind)),
		resource.IsManagedKind(resource.ManagedKind(cloudscalev1alpha1.S3BucketGroupVersionKind), mgr.GetScheme()),
	))

	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
		resource.ClassKind(cloudscalev1alpha1.S3BucketClassGroupVersionKind),
		resource.ManagedKind(cloudscalev1alpha1.S3BucketGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient(), mgr.GetScheme())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureS3Bucket),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &cloudscalev1alpha1.S3Bucket{}}, &resource.EnqueueRequestForClaim{}).
		For(&storagev1alpha1.Bucket{}).
		WithEventFilter(p).
		Complete(r)
}

// ConfigureS3Bucket configures the supplied resource (presumed
// to be a S3Bucket) using the supplied resource claim (presumed
// to be a Bucket) and resource class.
func ConfigureS3Bucket(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	bucketClaim, cmok := cm.(*storagev1alpha1.Bucket)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), storagev1alpha1.BucketGroupVersionKind)
	}

	s3BucketClass, csok := cs.(*cloudscalev1alpha1.S3BucketClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), cloudscalev1alpha1.S3BucketClassGroupVersionKind)
	}

	s3Bucket, mgok := mg.(*cloudscalev1alpha1.S3Bucket)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), cloudscalev1alpha1.S3BucketGroupVersionKind)
	}

	spec := &cloudscalev1alpha1.S3BucketSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		S3BucketParameters: s3BucketClass.SpecTemplate.S3BucketParameters,
	}

	if s3BucketClass.SpecTemplate.ReclaimPolicy != "" {
		spec.ResourceSpec.ReclaimPolicy = s3BucketClass.SpecTemplate.ReclaimPolicy
	}

	if bucketClaim.Spec.Name != "" {
		spec.NameFormat = bucketClaim.Spec.Name
	}

	spec.WriteConnectionSecretToReference = &runtimev1alpha1.SecretReference{
		Namespace: bucketClaim.Namespace, //s3BucketClass.SpecTemplate.WriteConnectionSecretsToNamespace,
		Name:      string(cm.GetUID()),
	}
	spec.ProviderReference = s3BucketClass.SpecTemplate.ProviderReference

	s3Bucket.Spec = *spec
	s3Bucket.Namespace = bucketClaim.Namespace

	return nil
}
