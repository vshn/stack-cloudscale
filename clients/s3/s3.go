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

	cloudscalev1alpha1 "git.vshn.net/syn/stack-cloudscale/api/v1alpha1"
)

// IsErrorNotFound helper function to test for BucketNotFound error
func IsErrorNotFound(err error) bool {
	if err != nil && err.Error() == "Not found" {
		return true
	}
	return false
}

// Service defines S3 Client operations
type Service interface {
	CreateOrUpdateBucket(ctx context.Context, bucket *cloudscalev1alpha1.S3Bucket) error
	GetBucketInfo(ctx context.Context, bucket *cloudscalev1alpha1.S3Bucket) (*cloudscalev1alpha1.S3Bucket, error)
	CreateUser(ctx context.Context, username string, bucket *cloudscalev1alpha1.S3Bucket) (string, string, error)
	DeleteBucket(ctx context.Context, bucket *cloudscalev1alpha1.S3Bucket) error
}

// Client implements S3 Client
type Client struct {
	credentials string
}

// NewClient creates a new S3 Client with provided Cloudscale credentials
func NewClient(ctx context.Context, credentials string) Service {
	return &Client{credentials: credentials}
}

// CreateOrUpdateBucket creates or updates the supplied S3 bucket with provided
// specification, and returns access keys with permissions of localPermission
func (c *Client) CreateOrUpdateBucket(ctx context.Context, bucket *cloudscalev1alpha1.S3Bucket) error {
	return nil
}

// GetBucketInfo returns the status of key bucket settings including user's policy version for permission status
func (c *Client) GetBucketInfo(ctx context.Context, bucket *cloudscalev1alpha1.S3Bucket) (*cloudscalev1alpha1.S3Bucket, error) {
	existing := &cloudscalev1alpha1.S3Bucket{
		Status: cloudscalev1alpha1.S3BucketStatus{
			Status: "Online",
		},
	}
	return existing, nil
}

// CreateUser - Create as user to access bucket per permissions in BucketSpec returing access key and policy version
func (c *Client) CreateUser(ctx context.Context, username string, bucket *cloudscalev1alpha1.S3Bucket) (string, string, error) {
	return "", "", nil
}

// DeleteBucket deletes s3 bucket, and related User
func (c *Client) DeleteBucket(ctx context.Context, bucket *cloudscalev1alpha1.S3Bucket) error {
	return nil
}
