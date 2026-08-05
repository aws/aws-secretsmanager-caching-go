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
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"
)

// The only version form this module releases.
var semVer = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// Version must match the released git tag, which is always MAJOR.MINOR.PATCH.
func TestVersionIsSemVer(t *testing.T) {
	if !semVer.MatchString(secretcache.Version) {
		t.Fatalf("Version %q is not a MAJOR.MINOR.PATCH semantic version", secretcache.Version)
	}
}

// Pins the pattern itself, which TestVersionIsSemVer cannot do on its own.
func TestSemVerPatternRejectsInvalid(t *testing.T) {
	invalid := map[string]string{
		"2.2.1.0": "the four-component format this package replaced",
		"2.2":     "too few components",
		"v2.2.1":  "the \"v\" prefix belongs to the git tag, not to Version",
		"02.1.0":  "leading zero",
		"2-2-1":   "components must be separated by dots",
	}

	for version, reason := range invalid {
		if semVer.MatchString(version) {
			t.Errorf("Expected %q to be rejected (%s)", version, reason)
		}
	}
}

// Returns the User-Agent header the cache sent. No AWS account is involved.
func captureUserAgent(t *testing.T) string {
	t.Helper()

	var userAgent string
	var requests int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		requests++

		// Writes an error to prevent retries
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"__type":"ResourceNotFoundException","message":"secret not found"}`))
	}))
	defer server.Close()

	// Redirect the SDK to the stub and pin what it would read from the host.
	t.Setenv("AWS_ENDPOINT_URL", server.URL)
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_SDK_UA_APP_ID", "")
	t.Setenv("AWS_EXECUTION_ENV", "")

	cache, err := secretcache.New()
	if err != nil {
		t.Fatalf("Unexpected error constructing cache - %s", err)
	}

	// Fails against the stub; the request that was sent is what matters.
	_, _ = cache.GetSecretString("dummy-secret-name")

	if requests == 0 {
		t.Fatal("No request reached the stub endpoint")
	}

	if userAgent == "" {
		t.Fatal("Request carried no User-Agent header")
	}

	return userAgent
}

// Requests must report "AwsSecretCache/<Version>", appended to the User-Agent.
func TestUserAgent(t *testing.T) {
	userAgent := captureUserAgent(t)

	want := "AwsSecretCache/" + secretcache.Version

	// A whole space-delimited entry, since a substring also matches "2.2.10".
	var found bool
	for _, entry := range strings.Fields(userAgent) {
		if entry == want {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected User-Agent to contain the entry %q, got %q", want, userAgent)
	}

	// Only the product name is pinned; the SDK's other metadata may change.
	if !strings.HasPrefix(userAgent, "aws-sdk-go-v2/") {
		t.Errorf("Expected User-Agent to still begin with the SDK product name, got %q", userAgent)
	}

	if strings.Index(userAgent, " "+want) < strings.Index(userAgent, "aws-sdk-go-v2/") {
		t.Errorf("Expected the cache entry to be appended after the SDK product name, got %q", userAgent)
	}
}
