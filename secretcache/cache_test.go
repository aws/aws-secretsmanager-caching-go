package secretcache_test

import (
	"bytes"
	"errors"
	"github.com/aws/aws-secretsmanager-caching-go/secretcache"
	"testing"

	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

func TestInstantiatesClient(t *testing.T) {
	secretCache, err := secretcache.New()

	if err != nil || secretCache.Client == nil {
		t.Fatalf("Failed to instantiate default Client")
	}
}

func TestGetSecretString(t *testing.T) {
	mockClient, _, secretString := newMockedClientWithDummyResults()
	secretCache, _ := secretcache.New(
			func(c *secretcache.Cache) {c.Client = &mockClient},
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
			func(c *secretcache.Cache) {c.Client = &mockClient},
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
	versionIdsToStages := make(map[string][]*string)
	versionIdsToStages["01234567890123456789012345678901"] = []*string{getStrPtr("AWSCURRENT")}

	mockClient := mockSecretsManagerClient{
		MockedGetResult:      &secretsmanager.GetSecretValueOutput{Name: getStrPtr("test")},
		MockedDescribeResult: &secretsmanager.DescribeSecretOutput{VersionIdsToStages: versionIdsToStages},
	}

	secretCache, _ := secretcache.New(
			func(c *secretcache.Cache) {c.Client = &mockClient},
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
	versionIdsToStages := make(map[string][]*string)
	versionIdsToStages["01234567890123456789012345678901"] = []*string{getStrPtr("NOT_CURRENT")}

	mockClient := mockSecretsManagerClient{
		MockedGetResult: &secretsmanager.GetSecretValueOutput{
			Name:         getStrPtr("test"),
			SecretString: getStrPtr("some secret string"),
			VersionId:    getStrPtr("01234567890123456789012345678901"),
		},
		MockedDescribeResult: &secretsmanager.DescribeSecretOutput{VersionIdsToStages: versionIdsToStages},
	}

	secretCache, _ := secretcache.New(
			func(c *secretcache.Cache) {c.Client = &mockClient},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
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

func TestGetSecretBinaryMultipleTimes(t *testing.T) {
	mockClient, secretId, _ := newMockedClientWithDummyResults()
	secretBinary := []byte{0, 1, 0, 1, 1, 1, 0, 0}
	mockClient.MockedGetResult.SecretBinary = secretBinary
	mockClient.MockedGetResult.SecretString = nil

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) {c.Client = &mockClient},
	)

	for i := 0; i < 10; i++ {
		result, err := secretCache.GetSecretBinary(secretId)
		if err != nil {
			t.Fatalf("Unexpected error - %s", err.Error())
		}

		if !bytes.Equal(result, secretBinary) {
			t.Fatalf("Expected and result binary are different")
		}
	}
}

func TestGetSecretStringRefresh(t *testing.T) {
	mockClient, secretId, secretString := newMockedClientWithDummyResults()

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) {c.Client = &mockClient},
		func(c *secretcache.Cache) {c.CacheConfig.CacheItemTTL = 1},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
		func(c *secretcache.Cache) {c.CacheConfig.CacheItemTTL = 1},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
	)

	for i := 0; i < 100; i++ {
		_, err := secretCache.GetSecretStringWithStage("test", "versionStage-42")

		if err == nil {
			t.Fatalf("Expected error for a missing secret")
		}
	}
}

func TestGetSecretBinaryMultipleNotFound(t *testing.T) {
	mockClient := mockSecretsManagerClient{
		GetSecretValueErr: errors.New("versionNotFound"),
		DescribeSecretErr: errors.New("secretNotFound"),
	}

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) {c.Client = &mockClient},
	)

	for i := 0; i < 100; i++ {
		_, err := secretCache.GetSecretBinaryWithStage("test", "versionStage-42")

		if err == nil {
			t.Fatalf("Expected error for a missing secret")
		}
	}
}

func TestGetSecretVersionStageEmpty(t *testing.T) {
	mockClient, _, secretString := newMockedClientWithDummyResults()

	secretCache, _ := secretcache.New(
		func(c *secretcache.Cache) {c.Client = &mockClient},
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
		func(c *secretcache.Cache) {c.Client = &mockClient},
		func(c *secretcache.Cache) {c.CacheConfig.VersionStage = ""},
	)

	result, err = secretCache.GetSecretStringWithStage("test", "")

	if err != nil {
		t.Fatalf("Unexpected error - %s", err.Error())
	}

	if result != secretString {
		t.Fatalf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, result)
	}
}
