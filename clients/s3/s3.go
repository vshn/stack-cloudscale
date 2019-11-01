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
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	cloudscale "github.com/cloudscale-ch/cloudscale-go-sdk"
)

// IsErrorNotFound helper function to test for BucketNotFound error
func IsErrorNotFound(err error) bool {
	if errResp, ok := err.(*cloudscale.ErrorResponse); ok {
		return errResp.StatusCode == 404
	} else if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()
		return code == s3.ErrCodeNoSuchBucket || code == "NotFound"
	}
	return false
}

// Service defines S3 Client operations
type Service interface {
	CreateOrUpdateBucket(ctx context.Context, userID, bucketName string, cannedACL *string, tags *map[string]string) (*cloudscale.ObjectUser, error)
	GetBucketInfo(ctx context.Context, userID, bucketName string) (*cloudscale.ObjectUser, error)
	DeleteBucket(ctx context.Context, userID, bucketName string) error
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
func (c *Client) CreateOrUpdateBucket(ctx context.Context, userID, bucketName string, cannedACL *string, tags *map[string]string) (*cloudscale.ObjectUser, error) {
	bucketTags := map[string]string{}
	if tags != nil {
		bucketTags = *tags
	}
	objectUserRequest := &cloudscale.ObjectUserRequest{
		DisplayName: bucketName,
		Tags:        bucketTags,
	}
	var objectUser *cloudscale.ObjectUser
	existingUser, err := c.getExistingBucketUser(ctx, userID, bucketName)
	if IsErrorNotFound(err) {
		objectUser, err = c.cloudscaleClient.ObjectUsers.Create(ctx, objectUserRequest)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		err := c.cloudscaleClient.ObjectUsers.Update(ctx, existingUser.ID, objectUserRequest)
		if err != nil {
			return nil, err
		}
		objectUser, err = c.getExistingBucketUser(ctx, userID, bucketName)
		if err != nil {
			return nil, err
		}
	}
	accessKey, secretKey, err := GetKeys(objectUser)
	if err != nil {
		return nil, err
	}
	err = createS3Bucket(bucketName, accessKey, secretKey, cannedACL)
	return objectUser, err
}

// GetBucketInfo returns the status of key bucket settings including user's policy version for permission status
func (c *Client) GetBucketInfo(ctx context.Context, userID, bucketName string) (*cloudscale.ObjectUser, error) {
	existingBucketUser, err := c.getExistingBucketUser(ctx, userID, bucketName)
	if err != nil {
		return nil, err
	}
	accessKey, secretKey, err := GetKeys(existingBucketUser)
	if err != nil {
		return nil, err
	}

	s3Client := getS3Client(accessKey, secretKey)
	hreq := &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	}
	_, err = s3Client.HeadBucketWithContext(ctx, hreq)

	if err != nil {
		return nil, err
	}

	return existingBucketUser, nil
}

// DeleteBucket deletes s3 bucket, and related User
func (c *Client) DeleteBucket(ctx context.Context, userID, bucketName string) error {
	existingBucketUser, err := c.getExistingBucketUser(ctx, userID, bucketName)
	if err != nil {
		return err
	}
	accessKey, secretKey, err := GetKeys(existingBucketUser)
	if err != nil {
		return err
	}
	err = deleteS3Bucket(bucketName, accessKey, secretKey)
	if err != nil {
		return err
	}
	return c.cloudscaleClient.ObjectUsers.Delete(ctx, existingBucketUser.ID)
}

func (c *Client) getExistingBucketUser(ctx context.Context, userID, bucketName string) (*cloudscale.ObjectUser, error) {
	if userID == "" {
		b, err := c.lookupUserByName(ctx, bucketName)
		if err != nil {
			return nil, err
		}
		userID = b.ID
	}
	return c.cloudscaleClient.ObjectUsers.Get(ctx, userID)
}

func createS3Bucket(bucketName, accessKey, secretKey string, cannedACL *string) error {
	acl := aws.String(s3.BucketCannedACLPrivate)
	if cannedACL != nil {
		acl = cannedACL
	}
	bucket := aws.String(bucketName)
	cparams := &s3.CreateBucketInput{
		Bucket: bucket,
		ACL:    acl,
	}
	s3Client := getS3Client(accessKey, secretKey)
	_, err := s3Client.CreateBucket(cparams)
	return err
}

func deleteS3Bucket(bucketName, accessKey, secretKey string) error {
	dparams := &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	}
	s3Client := getS3Client(accessKey, secretKey)
	_, err := s3Client.DeleteBucket(dparams)
	return err
}

func getS3Client(accessKey, secretKey string) *s3.S3 {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(cloudscale.S3Endpoint),
		Region:           aws.String("us-east-1"),
		DisableSSL:       aws.Bool(false),
		S3ForcePathStyle: aws.Bool(true),
	}
	newSession := session.New(s3Config)
	return s3.New(newSession)
}

func (c *Client) lookupUserByName(ctx context.Context, userName string) (*cloudscale.ObjectUser, error) {
	objectUsers, err := c.cloudscaleClient.ObjectUsers.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, user := range objectUsers {
		if userName == user.DisplayName {
			return &user, nil
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

// GetKeys returns the keys for a object user
func GetKeys(objectUser *cloudscale.ObjectUser) (string, string, error) {
	err := errors.New("Unexpected API return, keys found")
	if len(objectUser.Keys) != 1 {
		return "", "", err
	}
	accessKey, ok := objectUser.Keys[0]["access_key"]
	if !ok {
		return "", "", err
	}
	secretKey, ok := objectUser.Keys[0]["secret_key"]
	if !ok {
		return "", "", err
	}
	return accessKey, secretKey, nil
}
