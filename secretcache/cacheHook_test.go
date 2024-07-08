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

package secretcache_test

import (
	"bytes"
	"testing"

	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"
)

type DummyCacheHook struct {
	putCount int
	getCount int
}

func (hook *DummyCacheHook) Put(data interface{}) interface{} {
	hook.putCount++
	return data
}

func (hook *DummyCacheHook) Get(data interface{}) interface{} {
	hook.getCount++
	return data
}

func TestCacheHookString(t *testing.T) {
	mockClient, secretId, secretString := newMockedClientWithDummyResults()
	hook := &DummyCacheHook{}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
		func(c *secretcache.Cache) { c.CacheConfig.Hook = hook },
	)

	result, err := secretCache.GetSecretString(secretId)

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if result != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
	}

	if hook.getCount != 2 {
		t.Fatalf("Expected DummyCacheHook's get method to be called twice - once each for cacheItem and cacheVersion")
	}

	if hook.putCount != 2 {
		t.Fatalf("Expected DummyCacheHook's put method to be called twice - once each for cacheItem and cacheVersion")
	}
}

func TestCacheHookBinary(t *testing.T) {
	mockClient, secretId, _ := newMockedClientWithDummyResults()
	secretBinary := []byte{0, 0, 0, 0, 1, 1, 1, 1}
	mockClient.MockedGetResult.SecretString = nil
	mockClient.MockedGetResult.SecretBinary = secretBinary
	hook := &DummyCacheHook{}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
		func(c *secretcache.Cache) { c.CacheConfig.Hook = hook },
	)

	result, err := secretCache.GetSecretBinary(secretId)

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if !bytes.Equal(result, secretBinary) {
		t.Fatalf("Expected and result secret binary are different")
	}

	if hook.getCount != 2 {
		t.Fatalf("Expected DummyCacheHook's get method to be called twice - once each for cacheItem and cacheVersion")
	}

	if hook.putCount != 2 {
		t.Fatalf("Expected DummyCacheHook's put method to be called twice - once each for cacheItem and cacheVersion")
	}
}
