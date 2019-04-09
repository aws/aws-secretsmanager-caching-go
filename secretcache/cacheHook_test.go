package secretcache_test

import (
	"bytes"
	"github.com/aws/aws-secretsmanager-caching-go/secretcache"
	"testing"
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
		func(c *secretcache.Cache) {c.CacheConfig.Hook = hook},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
		func(c *secretcache.Cache) {c.CacheConfig.Hook = hook},
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
