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
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestInstantiatesClient(t *testing.T) {
	secretCache, err := secretcache.New()

	if err != nil || secretCache.Client == nil {
		t.Fatalf("Failed to instantiate default Client")
	}
}

func TestDefaultRequestTimeout(t *testing.T) {
	if secretcache.DefaultRequestTimeout != 2*time.Second {
		t.Fatalf("DefaultRequestTimeout is %s, expected 2s", secretcache.DefaultRequestTimeout)
	}

	isolateAWSEnv(t, "http://127.0.0.1:1", 1)
	secretCache, err := secretcache.New()
	if err != nil {
		t.Fatal(err)
	}
	httpClient, ok := secretCache.Client.(*secretsmanager.Client).Options().HTTPClient.(*awshttp.BuildableClient)
	if !ok {
		t.Fatal("Expected the default client to use the SDK's BuildableClient")
	}
	if httpClient.GetTimeout() != secretcache.DefaultRequestTimeout {
		t.Fatalf("Default client timeout is %s, expected %s", httpClient.GetTimeout(), secretcache.DefaultRequestTimeout)
	}
}

func TestDefaultClientGivesUpOnHungEndpoint(t *testing.T) {
	elapsed := timeToGiveUpOnHungEndpoint(t, 1)
	if elapsed < secretcache.DefaultRequestTimeout || elapsed > secretcache.DefaultRequestTimeout+3*time.Second {
		t.Fatalf("Took %s to give up, expected about %s", elapsed, secretcache.DefaultRequestTimeout)
	}
}

func TestDefaultClientRetriesTimedOutAttempt(t *testing.T) {
	// Two attempts, plus up to 2 seconds of backoff between them.
	elapsed := timeToGiveUpOnHungEndpoint(t, 2)
	if elapsed < 2*secretcache.DefaultRequestTimeout || elapsed > 2*secretcache.DefaultRequestTimeout+5*time.Second {
		t.Fatalf("Took %s to give up, expected two attempts of %s", elapsed, secretcache.DefaultRequestTimeout)
	}
}

// isolateAWSEnv points the default client at endpoint with dummy credentials, so
// the developer's own AWS settings (profile, endpoint overrides) can't change
// the test.
func isolateAWSEnv(t *testing.T, endpoint string, maxAttempts int) {
	t.Setenv("AWS_ENDPOINT_URL_SECRETS_MANAGER", endpoint)
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "")
	t.Setenv("AWS_USE_FIPS_ENDPOINT", "")
	t.Setenv("AWS_USE_DUALSTACK_ENDPOINT", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ACCESS_KEY_ID", "akid")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_CONFIG_FILE", os.DevNull)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", os.DevNull)
	t.Setenv("AWS_MAX_ATTEMPTS", strconv.Itoa(maxAttempts))
}

// timeToGiveUpOnHungEndpoint returns how long the default client takes to fail
// against an endpoint that hangs, and checks that it failed with a timeout.
func timeToGiveUpOnHungEndpoint(t *testing.T, maxAttempts int) time.Duration {
	// A listener that never accepts completes the TCP handshake but never
	// answers, like an endpoint that hangs after connecting.
	hung, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer hung.Close()

	isolateAWSEnv(t, "http://"+hung.Addr().String(), maxAttempts)
	secretCache, err := secretcache.New()
	if err != nil {
		t.Fatal(err)
	}

	// A guard deadline, so the test fails instead of hanging if the timeout is lost.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	_, err = secretCache.GetSecretStringWithContext(ctx, "test")
	elapsed := time.Since(start)

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() || ctx.Err() != nil {
		t.Fatalf("Expected the request's own timeout, got %v", err)
	}
	return elapsed
}

func TestGetSecretString(t *testing.T) {
	mockClient, _, secretString := newMockedClientWithDummyResults()
	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)
	result, err := secretCache.GetSecretString("test")

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if result != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
	}
}

func TestGetSecretBinary(t *testing.T) {
	mockClient, _, _ := newMockedClientWithDummyResults()
	secretBinary := []byte{0, 1, 1, 0, 0, 1, 1, 0}
	mockClient.MockedGetResult.SecretString = nil
	mockClient.MockedGetResult.SecretBinary = secretBinary
	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)
	result, err := secretCache.GetSecretBinary("test")

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if !bytes.Equal(result, secretBinary) {
		t.Fatalf("Expected and result secret binary are different ")
	}
}

func TestGetSecretMissing(t *testing.T) {
	versionIdsToStages := make(map[string][]string)
	versionIdsToStages["01234567890123456789012345678901"] = []string{"AWSCURRENT"}

	mockClient := mockSecretsManagerClient{
		MockedGetResult:      &secretsmanager.GetSecretValueOutput{Name: getStrPtr("test")},
		MockedDescribeResult: &secretsmanager.DescribeSecretOutput{VersionIdsToStages: versionIdsToStages},
	}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	_, err := secretCache.GetSecretString("test")

	if err == nil {
		t.Fatalf("Expected to not find a SecretString in this version")
	}

	_, err = secretCache.GetSecretBinary("test")

	if err == nil {
		t.Fatalf("Expected to not find a SecretString in this version")
	}
}

func TestGetSecretNoCurrent(t *testing.T) {
	versionIdsToStages := make(map[string][]string)
	versionIdsToStages["01234567890123456789012345678901"] = []string{"NOT_CURRENT"}

	mockClient := mockSecretsManagerClient{
		MockedGetResult: &secretsmanager.GetSecretValueOutput{
			Name:         getStrPtr("test"),
			SecretString: getStrPtr("some secret string"),
			VersionId:    getStrPtr("01234567890123456789012345678901"),
		},
		MockedDescribeResult: &secretsmanager.DescribeSecretOutput{VersionIdsToStages: versionIdsToStages},
	}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	_, err := secretCache.GetSecretString("test")

	if err == nil {
		t.Fatalf("Expected to not find secret version")
	}

	mockClient.MockedGetResult.SecretString = nil
	mockClient.MockedGetResult.SecretBinary = []byte{0, 1, 0, 1, 0, 1, 0, 1}

	_, err = secretCache.GetSecretBinary("test")

	if err == nil {
		t.Fatalf("Expected to not find secret version")
	}
}

func TestGetSecretVersionNotFound(t *testing.T) {

	mockClient, secretId, _ := newMockedClientWithDummyResults()
	mockClient.MockedGetResult = nil
	mockClient.GetSecretValueErr = errors.New("resourceNotFound")

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	_, err := secretCache.GetSecretString(secretId)

	if err == nil {
		t.Fatalf("Expected to not find secret version")
	}

	_, err = secretCache.GetSecretBinary(secretId)

	if err == nil {
		t.Fatalf("Expected to not find secret version")
	}
}

func TestGetSecretNoVersions(t *testing.T) {

	mockClient, secretId, _ := newMockedClientWithDummyResults()
	mockClient.MockedGetResult = nil
	mockClient.MockedDescribeResult.VersionIdsToStages = nil

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	_, err := secretCache.GetSecretString(secretId)

	if err == nil {
		t.Fatalf("Expected to not find secret version")
	}

	_, err = secretCache.GetSecretBinary(secretId)

	if err == nil {
		t.Fatalf("Expected to not find secret version")
	}
}

func TestGetSecretStringMultipleTimes(t *testing.T) {
	mockClient, secretId, secretString := newMockedClientWithDummyResults()
	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	for i := 0; i < 100; i++ {
		result, err := secretCache.GetSecretString(secretId)
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if result != secretString {
			t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
		}
	}

	if mockClient.DescribeSecretCallCount != 1 {
		t.Fatalf("Expected DescribeSecret to be called once, was called - \"%d\" times", mockClient.DescribeSecretCallCount)
	}

	if mockClient.GetSecretValueCallCount != 1 {
		t.Fatalf("Expected GetSecretValue to be called once, was called - \"%d\" times", mockClient.GetSecretValueCallCount)
	}
}

func TestGetSecretBinaryMultipleTimes(t *testing.T) {
	mockClient, secretId, _ := newMockedClientWithDummyResults()
	secretBinary := []byte{0, 1, 0, 1, 1, 1, 0, 0}
	mockClient.MockedGetResult.SecretBinary = secretBinary
	mockClient.MockedGetResult.SecretString = nil

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	for i := 0; i < 100; i++ {
		result, err := secretCache.GetSecretBinary(secretId)
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if !bytes.Equal(result, secretBinary) {
			t.Fatalf("Expected and result binary are different")
		}
	}

	if mockClient.DescribeSecretCallCount != 1 {
		t.Fatalf("Expected DescribeSecret to be called once, was called - \"%d\" times", mockClient.DescribeSecretCallCount)
	}

	if mockClient.GetSecretValueCallCount != 1 {
		t.Fatalf("Expected GetSecretValue to be called once, was called - \"%d\" times", mockClient.GetSecretValueCallCount)
	}
}

func TestGetSecretStringRefresh(t *testing.T) {
	mockClient, secretId, secretString := newMockedClientWithDummyResults()

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
		func(c *secretcache.Cache) { c.CacheConfig.CacheItemTTL = 1 },
	)

	for i := 0; i < 10; i++ {
		result, err := secretCache.GetSecretString(secretId)
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if result != secretString {
			t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
		}
	}
}

func TestGetSecretBinaryRefresh(t *testing.T) {
	mockClient, secretId, _ := newMockedClientWithDummyResults()
	secretBinary := []byte{0, 1, 1, 1, 1, 1, 0, 0}
	mockClient.MockedGetResult.SecretString = nil
	mockClient.MockedGetResult.SecretBinary = secretBinary

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
		func(c *secretcache.Cache) { c.CacheConfig.CacheItemTTL = 1 },
	)

	for i := 0; i < 10; i++ {
		result, err := secretCache.GetSecretBinary(secretId)
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if !bytes.Equal(result, secretBinary) {
			t.Fatalf("Expected and result secret binary are different")
		}
	}
}

func TestGetSecretStringWithStage(t *testing.T) {
	mockClient, secretId, secretString := newMockedClientWithDummyResults()

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	for i := 0; i < 10; i++ {
		result, err := secretCache.GetSecretStringWithStage(secretId, "versionStage-42")
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if result != secretString {
			t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
		}
	}
}

func TestGetSecretBinaryWithStage(t *testing.T) {
	mockClient, secretId, _ := newMockedClientWithDummyResults()
	secretBinary := []byte{0, 1, 1, 0, 0, 1, 0, 1}
	mockClient.MockedGetResult.SecretString = nil
	mockClient.MockedGetResult.SecretBinary = secretBinary

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	for i := 0; i < 10; i++ {
		result, err := secretCache.GetSecretBinaryWithStage(secretId, "versionStage-42")
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if !bytes.Equal(result, secretBinary) {
			t.Fatalf("Expected and result secret binary are different")
		}
	}
}

func TestGetSecretStringMultipleNotFound(t *testing.T) {
	mockClient := mockSecretsManagerClient{
		GetSecretValueErr: errors.New("versionNotFound"),
		DescribeSecretErr: errors.New("secretNotFound"),
	}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	for i := 0; i < 100; i++ {
		_, err := secretCache.GetSecretStringWithStage("test", "versionStage-42")

		if err == nil {
			t.Fatalf("Expected error: secretNotFound for a missing secret")
		}
	}

	if mockClient.DescribeSecretCallCount != 1 {
		t.Fatalf("Expected a single call to DescribeSecret API, got %d", mockClient.DescribeSecretCallCount)
	}
}

func TestGetSecretBinaryMultipleNotFound(t *testing.T) {
	mockClient := mockSecretsManagerClient{
		GetSecretValueErr: errors.New("versionNotFound"),
		DescribeSecretErr: errors.New("secretNotFound"),
	}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	for i := 0; i < 100; i++ {
		_, err := secretCache.GetSecretBinaryWithStage("test", "versionStage-42")

		if err == nil {
			t.Fatalf("Expected error: secretNotFound for a missing secret")
		}
	}

	if mockClient.DescribeSecretCallCount != 1 {
		t.Fatalf("Expected a single call to DescribeSecret API, got %d", mockClient.DescribeSecretCallCount)
	}
}

func TestRefreshNow(t *testing.T) {
	mockClient, secretId, secretString := newMockedClientWithDummyResults()
	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
		func(c *secretcache.Cache) { c.CacheConfig.CacheItemTTL = time.Hour.Nanoseconds() },
	)
	originalSecret, err := secretCache.GetSecretString(secretId)
	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}
	if originalSecret != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, originalSecret)
	}

	_, _ = secretCache.GetSecretString(secretId)

	if mockClient.DescribeSecretCallCount != 1 {
		t.Fatalf("Expected a single call to DescribeSecret API, got %d", mockClient.DescribeSecretCallCount)
	}

	secretCache.RefreshNow(secretId)
	refreshedSecret, err := secretCache.GetSecretString(secretId)

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if refreshedSecret != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, refreshedSecret)
	}

	if mockClient.DescribeSecretCallCount != 2 {
		t.Fatalf("Expected two calls to DescribeSecret API, got %d", mockClient.DescribeSecretCallCount)
	}

	_, _ = secretCache.GetSecretString(secretId)

	if mockClient.DescribeSecretCallCount != 2 {
		t.Fatalf("Expected two calls to DescribeSecret API, got %d", mockClient.DescribeSecretCallCount)
	}

}

func TestGetSecretVersionStageEmpty(t *testing.T) {
	mockClient, _, secretString := newMockedClientWithDummyResults()

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
	)

	result, err := secretCache.GetSecretStringWithStage("test", "")

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if result != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
	}

	//New cache for new config
	secretCache, _ = secretcache.New(
		func(c *secretcache.Cache) { c.Client = &mockClient },
		func(c *secretcache.Cache) { c.CacheConfig.VersionStage = "" },
	)

	result, err = secretCache.GetSecretStringWithStage("test", "")

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if result != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
	}
}
