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
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vshn/stack-cloudscale/controllers/s3"
)

// SetupWithManager adds all Cloudscale controllers to the manager.
func SetupWithManager(mgr ctrl.Manager) error {
	controllers := []interface {
		SetupWithManager(ctrl.Manager) error
	}{
		&s3.BucketClaimSchedulingController{},
		&s3.BucketClaimDefaultingController{},
		&s3.BucketClaimController{},
		&s3.BucketInstanceController{},
	}

	for _, c := range controllers {
		if err := c.SetupWithManager(mgr); err != nil {
			return err
		}
	}
	return nil
}
