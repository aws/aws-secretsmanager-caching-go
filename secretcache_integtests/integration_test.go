package secretcache_integtests

import (
	"bytes"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-secretsmanager-caching-go/secretcache"
	"math/rand"
	"testing"
	"time"
)

var (
	randStringSet   = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	subTests        = []func(t *testing.T, api secretsmanageriface.SecretsManagerAPI) string{
		integTest_getSecretBinary,
		integTest_getSecretBinaryWithStage,
		integTest_getSecretString,
		integTest_getSecretStringWithStage,
		integTest_getSecretStringWithTTL,
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
	secretName := testName + "-" + clientRequestToken

	return secretName, clientRequestToken
}

func createSecret(
	testName string, secretString *string, secretBinary []byte, api secretsmanageriface.SecretsManagerAPI,
) (*secretsmanager.CreateSecretOutput, error) {
	secretName, requestToken := generateSecretName(testName)
	createSecretInput := &secretsmanager.CreateSecretInput{
		Name:               &secretName,
		SecretString:       secretString,
		SecretBinary:       secretBinary,
		ClientRequestToken: &requestToken,
	}
	return api.CreateSecret(createSecretInput)
}

func TestIntegration(t *testing.T) {

	// Create a new API client
	// See https://docs.aws.amazon.com/sdk-for-go/api/aws/session/ for how the session loads credentials
	sess, err := session.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	secretsManagerClient := secretsmanager.New(sess)

	// Run all integ tests and collect the secret arns created for them
	var secretNames []string
	for _, testFunc := range subTests {
		secretNames = append(secretNames, testFunc(t, secretsManagerClient))
	}

	// Lazily delete all the secrets we created
	var true = true
	for _, secretName := range secretNames {
		time.Sleep(time.Second / 2)
		_, _ = secretsManagerClient.DeleteSecret(&secretsmanager.DeleteSecretInput{
			SecretId:                   &secretName,
			ForceDeleteWithoutRecovery: &true,
		})
	}
}

func integTest_getSecretBinary(t *testing.T, api secretsmanageriface.SecretsManagerAPI) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretBinary := []byte{0, 1, 1, 0, 0, 1, 1, 0}
	createResult, err := createSecret("integTest_getSecretBinary", nil, secretBinary, api)

	if err != nil {
		t.Errorf("Failed to create secret \"integTest_getSecretBinary\" ERROR: %s", err)
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

func integTest_getSecretBinaryWithStage(t *testing.T, api secretsmanageriface.SecretsManagerAPI) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretBinary := []byte{0, 1, 1, 0, 0, 1, 1, 0}
	createResult, err := createSecret("integTest_getSecretBinaryWithStage", nil, secretBinary, api)

	if err != nil {
		t.Errorf("Failed to create secret \"integTest_getSecretBinaryWithStage\" ERROR: %s", err)
		return ""
	}

	updatedSecretBinary := []byte{1, 0, 0, 1, 1, 0, 0, 1}
	updatedRequestToken := generateRandString(32)
	_, err = api.UpdateSecret(&secretsmanager.UpdateSecretInput{
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

func integTest_getSecretString(t *testing.T, api secretsmanageriface.SecretsManagerAPI) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)
	secretString := "This is a secret"
	createResult, err := createSecret("test_getSecretString", &secretString, nil, api)

	if err != nil {
		t.Errorf("Failed to create secret: \"test_getSecretString\" ERROR: %s", err)
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

func integTest_getSecretStringWithStage(t *testing.T, api secretsmanageriface.SecretsManagerAPI) string {
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
	)

	secretString := "This is a secret string"
	createResult, err := createSecret("integTest_getSecretStringWithStage", &secretString, nil, api)

	if err != nil {
		t.Error("Failed to create secret ", err)
		return ""
	}

	updatedSecretString := "This is v2 secret string"
	updatedRequestToken := generateRandString(32)
	_, err = api.UpdateSecret(&secretsmanager.UpdateSecretInput{
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

func integTest_getSecretStringWithTTL(t *testing.T, api secretsmanageriface.SecretsManagerAPI) string {
	ttlNanoSeconds := (time.Second * 2).Nanoseconds()
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.Client = api },
		func(c *secretcache.Cache) { c.CacheItemTTL = ttlNanoSeconds },
	)

	secretString := "This is a secret"
	createResult, err := createSecret("integTest_getSecretStringWithTTL", &secretString, nil, api)

	if err != nil {
		t.Errorf("Failed to create secret: \"test_getSecretString\" ERROR: %s", err)
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
	_, err = api.UpdateSecret(&secretsmanager.UpdateSecretInput{
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
	if updatedSecretString != resultString {
		t.Errorf("Expected cached secret to be same as updated version - \"%s\", \"%s\"", resultString, updatedSecretString)
		return *createResult.ARN
	}

	return *createResult.ARN
}
