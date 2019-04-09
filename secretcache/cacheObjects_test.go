// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You
// may not use this file except in compliance with the License. A copy of
// the License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is
// distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific
// language governing permissions and limitations under the License.

package secretcache

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

func TestIsRefreshNeededBase(t *testing.T) {
	obj := cacheObject{refreshNeeded: true}

	if !obj.isRefreshNeeded() {
		t.Fatalf("Expected true when refreshNeeded is true")
	}

	obj.refreshNeeded = false

	if obj.isRefreshNeeded() {
		t.Fatalf("Expected false when err is nil")
	}

	obj.err = errors.New("some dummy error")

	if !obj.isRefreshNeeded() {
		t.Fatalf("Expected true when err is not nil")
	}

	obj.nextRetryTime = time.Now().Add(time.Hour * 1).UnixNano()

	if obj.isRefreshNeeded() {
		t.Fatalf("Expected false when nextRetryTime is in future")
	}

	obj.nextRetryTime = time.Now().Add(-(time.Hour * 1)).UnixNano()
	if !obj.isRefreshNeeded() {
		t.Fatalf("Expected true when nextRetryTime is in past")
	}
}

func TestMaxCacheTTL(t *testing.T) {

	mockClient := dummyClient{}

	cacheItem := secretCacheItem{
		cacheObject: &cacheObject{
			secretId: "dummy-secret-name",
			client:   &mockClient,
			data: &secretsmanager.DescribeSecretOutput{
				ARN:         getStrPtr("dummy-arn"),
				Name:        getStrPtr("dummy-name"),
				Description: getStrPtr("dummy-description"),
			},
		},
	}

	config := CacheConfig{CacheItemTTL: -1}
	cacheItem.config = config

	_, err := cacheItem.executeRefresh()

	if err == nil {
		t.Fatalf("Expected error due to negative cache ttl")
	}

	config = CacheConfig{CacheItemTTL: 0}
	cacheItem.config = config

	_, err = cacheItem.executeRefresh()

	if err != nil {
		t.Fatalf("Unexpected error on zero cache ttl")
	}
}

type dummyClient struct {
	secretsmanageriface.SecretsManagerAPI
}

func (d *dummyClient) DescribeSecretWithContext(context aws.Context, input *secretsmanager.DescribeSecretInput, opts ...request.Option) (*secretsmanager.DescribeSecretOutput, error) {
	return &secretsmanager.DescribeSecretOutput{}, nil
}

// Helper function to get a string pointer for input string.
func getStrPtr(str string) *string {
	return &str
}
