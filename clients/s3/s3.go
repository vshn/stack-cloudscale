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
	"net/http"

	"github.com/pkg/errors"

	cloudscale "github.com/cloudscale-ch/cloudscale-go-sdk"
)

// BucketInfo shows info about a bucket and it's user
type BucketInfo struct {
	BucketName string
	UserID     string
	AccessKey  string
	SecretKey  string
	Tags       map[string]string
	Endpoint   string
}

// IsErrorNotFound helper function to test for BucketNotFound error
func IsErrorNotFound(err error) bool {
	if errResp, ok := err.(*cloudscale.ErrorResponse); ok {
		return errResp.StatusCode == 404
	}
	return false
}

// Service defines S3 Client operations
type Service interface {
	CreateOrUpdateBucket(ctx context.Context, bucket BucketInfo) (*BucketInfo, error)
	GetBucketInfo(ctx context.Context, bucket BucketInfo) (*BucketInfo, error)
	DeleteBucket(ctx context.Context, bucket BucketInfo) error
}

// Client implements S3 Client
type Client struct {
	cloudscaleClient *cloudscale.Client
}

// NewClient creates a new S3 Client with provided Cloudscale credentials
func NewClient(ctx context.Context, cloudscaleToken string, httpClient *http.Client) Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	c := &Client{
		cloudscaleClient: cloudscale.NewClient(httpClient),
	}
	c.cloudscaleClient.AuthToken = cloudscaleToken

	return c
}

// CreateOrUpdateBucket creates or updates the supplied S3 bucket with provided
// specification
func (c *Client) CreateOrUpdateBucket(ctx context.Context, bucket BucketInfo) (*BucketInfo, error) {
	objectUserRequest := &cloudscale.ObjectUserRequest{
		DisplayName: bucket.BucketName,
		Tags:        bucket.Tags,
	}
	existingUser, err := c.GetBucketInfo(ctx, bucket)
	if IsErrorNotFound(err) {
		objectUser, err := c.cloudscaleClient.ObjectUsers.Create(ctx, objectUserRequest)
		if err != nil {
			return nil, err
		}
		return toBucketInfo(objectUser)
	} else if err != nil {
		return nil, err
	} else {
		err := c.cloudscaleClient.ObjectUsers.Update(ctx, existingUser.UserID, objectUserRequest)
		if err != nil {
			return nil, err
		}
		return c.GetBucketInfo(ctx, bucket)
	}
}

// GetBucketInfo returns the status of key bucket settings including user's policy version for permission status
func (c *Client) GetBucketInfo(ctx context.Context, bucket BucketInfo) (*BucketInfo, error) {
	if bucket.UserID == "" {
		return c.lookupUserByName(ctx, bucket.BucketName)
	}
	bucketUser, err := c.cloudscaleClient.ObjectUsers.Get(ctx, bucket.UserID)
	if err != nil {
		return nil, err
	}
	return toBucketInfo(bucketUser)
}

// DeleteBucket deletes s3 bucket, and related User
func (c *Client) DeleteBucket(ctx context.Context, bucket BucketInfo) error {
	err := c.cloudscaleClient.ObjectUsers.Delete(ctx, bucket.UserID)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) lookupUserByName(ctx context.Context, userName string) (*BucketInfo, error) {
	objectUsers, err := c.cloudscaleClient.ObjectUsers.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, user := range objectUsers {
		if userName == user.DisplayName {
			return toBucketInfo(&user)
		}
	}
	err = &cloudscale.ErrorResponse{
		StatusCode: 404,
		Message: map[string]string{
			"Error": "User not found",
		},
	}
	return nil, err
}

func toBucketInfo(objectUser *cloudscale.ObjectUser) (*BucketInfo, error) {
	err := errors.New("Unexpected keys found")
	if len(objectUser.Keys) != 1 {
		return nil, err
	}
	accessKey, ok := objectUser.Keys[0]["access_key"]
	if !ok {
		return nil, err
	}
	secretKey, ok := objectUser.Keys[0]["secret_key"]
	if !ok {
		return nil, err
	}
	bInfo := &BucketInfo{
		UserID:     objectUser.ID,
		Tags:       objectUser.Tags,
		BucketName: objectUser.DisplayName,
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		Endpoint:   cloudscale.S3Endpoint,
	}

	return bInfo, nil
}
