// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"
)

type stageSelectionClient struct {
	secretcache.SecretsManagerAPIClient
	binary            bool
	onlyPrevious      bool
	getErr            error
	requestedVersions []string
	describeContext   context.Context
	getContext        context.Context
}

func (c *stageSelectionClient) DescribeSecret(ctx context.Context, _ *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	c.describeContext = ctx
	stages := map[string][]string{"previous-version": {"AWSPREVIOUS"}}
	if !c.onlyPrevious {
		stages["current-version"] = []string{"AWSCURRENT"}
	}
	return &secretsmanager.DescribeSecretOutput{VersionIdsToStages: stages}, nil
}

func (c *stageSelectionClient) GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	c.getContext = ctx
	version := aws.ToString(input.VersionId)
	c.requestedVersions = append(c.requestedVersions, version)
	if c.getErr != nil {
		return nil, c.getErr
	}
	output := &secretsmanager.GetSecretValueOutput{VersionId: input.VersionId}
	if c.binary {
		output.SecretBinary = []byte("value-for-" + version)
	} else {
		output.SecretString = aws.String("value-for-" + version)
	}
	return output, nil
}

func TestVersionStageSelection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestErr := errors.New("GetSecretValue failed")
	type stageCase struct {
		name         string
		configured   *string
		explicit     string
		wantVersion  string
		onlyPrevious bool
		getErr       error
		missing      bool
	}
	ordinaryCases := []stageCase{
		{name: "default", wantVersion: "current-version"},
		{name: "configured", configured: aws.String("AWSPREVIOUS"), wantVersion: "previous-version"},
		{name: "empty_config", configured: aws.String(""), wantVersion: "current-version"},
		{name: "without_current", configured: aws.String("AWSPREVIOUS"), wantVersion: "previous-version", onlyPrevious: true},
		{name: "missing_configured_stage", configured: aws.String("missing"), missing: true},
		{name: "request_error", configured: aws.String("AWSPREVIOUS"), wantVersion: "previous-version", getErr: requestErr},
	}
	explicitCases := []stageCase{
		{name: "override_default", explicit: "AWSPREVIOUS", wantVersion: "previous-version"},
		{name: "override_configured", configured: aws.String("AWSPREVIOUS"), explicit: "AWSCURRENT", wantVersion: "current-version"},
		{name: "empty_uses_configured", configured: aws.String("AWSPREVIOUS"), wantVersion: "previous-version"},
		{name: "empty_config_and_stage", configured: aws.String(""), wantVersion: "current-version"},
		{name: "missing_explicit_stage", explicit: "missing", missing: true},
		{name: "request_error", explicit: "AWSPREVIOUS", wantVersion: "previous-version", getErr: requestErr},
	}
	getters := []struct {
		name        string
		binary      bool
		withContext bool
		explicit    bool
		get         func(*secretcache.Cache, string) (string, error)
	}{
		{name: "string", get: func(c *secretcache.Cache, _ string) (string, error) { return c.GetSecretString("test") }},
		{name: "string_context", withContext: true, get: func(c *secretcache.Cache, _ string) (string, error) { return c.GetSecretStringWithContext(ctx, "test") }},
		{name: "binary", binary: true, get: func(c *secretcache.Cache, _ string) (string, error) {
			v, err := c.GetSecretBinary("test")
			return string(v), err
		}},
		{name: "binary_context", binary: true, withContext: true, get: func(c *secretcache.Cache, _ string) (string, error) {
			v, err := c.GetSecretBinaryWithContext(ctx, "test")
			return string(v), err
		}},
		{name: "string_stage", explicit: true, get: func(c *secretcache.Cache, stage string) (string, error) {
			return c.GetSecretStringWithStage("test", stage)
		}},
		{name: "string_stage_context", explicit: true, withContext: true, get: func(c *secretcache.Cache, stage string) (string, error) {
			return c.GetSecretStringWithStageWithContext(ctx, "test", stage)
		}},
		{name: "binary_stage", binary: true, explicit: true, get: func(c *secretcache.Cache, stage string) (string, error) {
			v, err := c.GetSecretBinaryWithStage("test", stage)
			return string(v), err
		}},
		{name: "binary_stage_context", binary: true, explicit: true, withContext: true, get: func(c *secretcache.Cache, stage string) (string, error) {
			v, err := c.GetSecretBinaryWithStageWithContext(ctx, "test", stage)
			return string(v), err
		}},
	}
	for _, getter := range getters {
		t.Run(getter.name, func(t *testing.T) {
			cases := ordinaryCases
			if getter.explicit {
				cases = explicitCases
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					client := &stageSelectionClient{binary: getter.binary, onlyPrevious: tc.onlyPrevious, getErr: tc.getErr}
					cache, err := secretcache.New(func(c *secretcache.Cache) {
						c.Client = client
						if tc.configured != nil {
							c.VersionStage = *tc.configured
						}
					})
					if err != nil {
						t.Fatal(err)
					}
					value, err := getter.get(cache, tc.explicit)
					if tc.missing {
						var missing *secretcache.VersionNotFoundError
						if !errors.As(err, &missing) || value != "" || len(client.requestedVersions) != 0 {
							t.Fatalf("missing stage: value=%q err=%v requests=%v", value, err, client.requestedVersions)
						}
						return
					}
					if len(client.requestedVersions) != 1 || client.requestedVersions[0] != tc.wantVersion {
						t.Fatalf("requested versions = %v, want [%s]", client.requestedVersions, tc.wantVersion)
					}
					if getter.withContext && (client.describeContext != ctx || client.getContext != ctx) {
						t.Fatal("context was not forwarded to the client")
					}
					if tc.getErr != nil {
						if !errors.Is(err, tc.getErr) || value != "" {
							t.Fatalf("request failure: value=%q err=%v, want %v", value, err, tc.getErr)
						}
						return
					}
					want := "value-for-" + tc.wantVersion
					if err != nil || value != want {
						t.Fatalf("got (%q, %v), want (%q, nil)", value, err, want)
					}
					value, err = getter.get(cache, tc.explicit)
					if err != nil || value != want || len(client.requestedVersions) != 1 {
						t.Fatalf("cached read: value=%q err=%v requests=%v", value, err, client.requestedVersions)
					}
				})
			}
		})
	}
}
