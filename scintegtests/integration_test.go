package scintegtests

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"regexp"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"
	"github.com/aws/smithy-go"
)

var (
	randStringSet    = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	secretNamePrefix = "scIntegTest_"
	subTests         = []func(t *testing.T, api secretcache.SecretsManagerAPIClient) string{
		integTest_getSecretBinary,
		integTest_getSecretBinaryWithStage,
		integTest_getSecretString,
		integTest_getSecretStringWithStage,
		integTest_getSecretStringWithTTL,
		integTest_getSecretStringNoSecret,
	}
)

func init() {
	rand.Seed(time.Now().Unix())
}

func generateRandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = randStringSet[rand.Intn(len(randStringSet))]
	}
	return string(b)
}

func generateSecretName(testName string) (string, string) {
	clientRequestToken := generateRandString(32)
	secretName := secretNamePrefix + testName + "_" + clientRequestToken

	return secretName, clientRequestToken
}

func createSecret(
	testName string, secretString *string, secretBinary []byte, api secretcache.SecretsManagerAPIClient,
) (*secretsmanager.CreateSecretOutput, error) {
	secretName, requestToken := generateSecretName(testName)
	createSecretInput := &secretsmanager.CreateSecretInput{
		Name:               &secretName,
		SecretString:       secretString,
		SecretBinary:       secretBinary,
		ClientRequestToken: &requestToken,
	}
	return api.CreateSecret(context.TODO(), createSecretInput)
}

// Lazily delete all the secrets we created
// Also delete secrets created over 2 days ago, with the "scIntegTest_" prefix
func cleanupSecrets(secretNames *[]string, secretsManagerClient secretcache.SecretsManagerAPIClient, t *testing.T) {

	// Cleanup secrets created on this test run
	performDelete(secretNames, secretsManagerClient, true)

	prevRunSecrets := getPrevRunSecrets(secretsManagerClient)
	for _, secretARN := range prevRunSecrets {
		t.Logf("Scheduling deletion for secret: \"%s\"", secretARN)
	}

	// Cleanup secrets created on past runs
	performDelete(&prevRunSecrets, secretsManagerClient, false)
}

func getPrevRunSecrets(secretsManagerClient secretcache.SecretsManagerAPIClient) []string {
	var nextToken *string
	var secretNames []string
	twoDaysAgo := time.Now().Add(-(48 * time.Hour))
	testSecretNamePrefix := "^" + secretNamePrefix + ".+"

	for {
		resp, err := secretsManagerClient.ListSecrets(
			context.TODO(),
			&secretsmanager.ListSecretsInput{NextToken: nextToken},
		)

		if resp == nil || err != nil {
			break
		}

		for _, secret := range resp.SecretList {
			var name []byte
			copy(name, *secret.Name)
			match, _ := regexp.Match(testSecretNamePrefix, name)
			if match && secret.LastChangedDate.Before(twoDaysAgo) && secret.LastAccessedDate.Before(twoDaysAgo) {
				secretNames = append(secretNames, *secret.ARN)
			}
		}

		if resp.NextToken == nil {
			break
		}

		nextToken = resp.NextToken
		time.Sleep(1 * time.Second)
	}
	return secretNames
}

func performDelete(secretNames *[]string, secretsManagerClient secretcache.SecretsManagerAPIClient, forceDelete bool) {
	for _, secretName := range *secretNames {

		if secretName == "" {
			continue
		}

		time.Sleep(time.Second / 2)
		_, _ = secretsManagerClient.DeleteSecret(
			context.TODO(),
			&secretsmanager.DeleteSecretInput{
				SecretId:                   &secretName,
				ForceDeleteWithoutRecovery: &forceDelete,
			})
	}
}

func TestIntegration(t *testing.T) {

	// Create a new API client
	// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/ for how the config loads credentials
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	secretsManagerClient := secretsmanager.NewFromConfig(cfg)

	// Collect the secret arns created for them
	var secretNames []string

	// Defer cleanup of secrets to ensure cleanup in case of caller function being terminated
	defer cleanupSecrets(&secretNames, secretsManagerClient, t)

	// Run integ tests
	for _, testFunc := range subTests {
		secretNames = append(secretNames, testFunc(t, secretsManagerClient))
	}
}

func integTest_getSecretBinary(t *testing.T, api secretcache.SecretsManagerAPIClient) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretBinary := []byte{0, 1, 1, 0, 0, 1, 1, 0}
	createResult, err := createSecret("getSecretBinary", nil, secretBinary, api)

	if err != nil {
		t.Errorf("Failed to create secret \"getSecretBinary\" ERROR: %s", err)
		return ""
	}

	resultBinary, err := cache.GetSecretBinary(*createResult.ARN)

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if !bytes.Equal(resultBinary, secretBinary) {
		t.Error("Expected and result binary not the same")
	}

	return *createResult.ARN
}

func integTest_getSecretBinaryWithStage(t *testing.T, api secretcache.SecretsManagerAPIClient) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretBinary := []byte{0, 1, 1, 0, 0, 1, 1, 0}
	createResult, err := createSecret("getSecretBinaryWithStage", nil, secretBinary, api)

	if err != nil {
		t.Errorf("Failed to create secret \"getSecretBinaryWithStage\" ERROR: %s", err)
		return ""
	}

	updatedSecretBinary := []byte{1, 0, 0, 1, 1, 0, 0, 1}
	updatedRequestToken := generateRandString(32)
	_, err = api.UpdateSecret(
		context.TODO(),
		&secretsmanager.UpdateSecretInput{
			SecretId:           createResult.ARN,
			SecretBinary:       updatedSecretBinary,
			ClientRequestToken: &updatedRequestToken,
		})

	if err != nil {
		t.Errorf("Failed to update secret: \"%s\" ERROR: %s", *createResult.ARN, err)
		return *createResult.ARN
	}

	resultBinary, err := cache.GetSecretBinaryWithStage(*createResult.ARN, "AWSPREVIOUS")

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if !bytes.Equal(resultBinary, secretBinary) {
		t.Error("Expected and result binary not the same")
	}

	resultBinary, err = cache.GetSecretBinaryWithStage(*createResult.ARN, "AWSCURRENT")

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if !bytes.Equal(resultBinary, updatedSecretBinary) {
		t.Error("Expected and result binary not the same")
	}

	return *createResult.ARN
}

func integTest_getSecretString(t *testing.T, api secretcache.SecretsManagerAPIClient) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)
	secretString := "This is a secret"
	createResult, err := createSecret("getSecretString", &secretString, nil, api)

	if err != nil {
		t.Errorf("Failed to create secret: \"getSecretString\" ERROR: %s", err)
		return ""
	}

	resultString, err := cache.GetSecretString(*createResult.ARN)

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if secretString != resultString {
		t.Errorf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, resultString)
	}

	return *createResult.ARN
}

func integTest_getSecretStringWithStage(t *testing.T, api secretcache.SecretsManagerAPIClient) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretString := "This is a secret string"
	createResult, err := createSecret("getSecretStringWithStage", &secretString, nil, api)

	if err != nil {
		t.Errorf("Failed to create secret: \"getSecretStringWithStage\" ERROR: %s", err)
		return ""
	}

	updatedSecretString := "This is v2 secret string"
	updatedRequestToken := generateRandString(32)
	_, err = api.UpdateSecret(
		context.TODO(),
		&secretsmanager.UpdateSecretInput{
			SecretId:           createResult.ARN,
			SecretString:       &updatedSecretString,
			ClientRequestToken: &updatedRequestToken,
		})

	if err != nil {
		t.Errorf("Failed to update secret: \"%s\" ERROR: %s", *createResult.ARN, err)
		return *createResult.ARN
	}

	resultString, err := cache.GetSecretStringWithStage(*createResult.ARN, "AWSPREVIOUS")

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if secretString != resultString {
		t.Errorf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, resultString)
	}

	resultString, err = cache.GetSecretStringWithStage(*createResult.ARN, "AWSCURRENT")

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if resultString != updatedSecretString {
		t.Errorf("Expected and result secret string are different - \"%s\", \"%s\"", updatedSecretString, resultString)
	}

	return *createResult.ARN
}

func integTest_getSecretStringWithTTL(t *testing.T, api secretcache.SecretsManagerAPIClient) string {
	ttlNanoSeconds := (time.Second * 2).Nanoseconds()
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
		func(c *secretcache.Cache) { c.CacheItemTTL = ttlNanoSeconds },
	)

	secretString := "This is a secret"
	createResult, err := createSecret("getSecretStringWithTTL", &secretString, nil, api)

	if err != nil {
		t.Errorf("Failed to create secret: \"getSecretStringWithTTL\" ERROR: %s", err)
		return ""
	}

	resultString, err := cache.GetSecretString(*createResult.ARN)

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if secretString != resultString {
		t.Errorf("Expected and result secret string are different - \"%s\", \"%s\"", secretString, resultString)
		return *createResult.ARN
	}

	updatedSecretString := "This is v2 secret string"
	updatedRequestToken := generateRandString(32)
	_, err = api.UpdateSecret(
		context.TODO(),
		&secretsmanager.UpdateSecretInput{
			SecretId:           createResult.ARN,
			SecretString:       &updatedSecretString,
			ClientRequestToken: &updatedRequestToken,
		})

	if err != nil {
		t.Errorf("Failed to update secret: \"%s\" ERROR: %s", *createResult.ARN, err)
		return *createResult.ARN
	}

	resultString, err = cache.GetSecretString(*createResult.ARN)

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if secretString != resultString {
		t.Errorf("Expected cached secret to be same as previous version - \"%s\", \"%s\"", resultString, secretString)
		return *createResult.ARN
	}

	time.Sleep(time.Nanosecond * time.Duration(ttlNanoSeconds))

	resultString, err = cache.GetSecretString(*createResult.ARN)

	if err != nil {
		t.Error(err)
		return *createResult.ARN
	}

	if updatedSecretString != resultString {
		t.Errorf("Expected cached secret to be same as updated version - \"%s\", \"%s\"", resultString, updatedSecretString)
		return *createResult.ARN
	}

	return *createResult.ARN
}

func integTest_getSecretStringNoSecret(t *testing.T, api secretcache.SecretsManagerAPIClient) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretName := "NoSuchSecret"
	_, err := cache.GetSecretString(secretName)

	var apiErr smithy.APIError

	if err == nil {
		t.Errorf("Expected to not find a secret called %s", secretName)
	} else if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() != ErrCodeResourceNotFoundException {
			t.Errorf("Expected %s err but got %s", ErrCodeResourceNotFoundException, apiErr.ErrorCode())
		}
	}

	return ""
}
