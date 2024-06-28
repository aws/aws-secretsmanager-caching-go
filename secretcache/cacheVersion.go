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
	"context"
	"math"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// cacheVersion is the cache object for a secret version.
type cacheVersion struct {
	versionId string
	*cacheObject
}

// newCacheVersion initialises a cacheVersion to cache a secret version.
func newCacheVersion(config CacheConfig, client SecretsManagerAPIClient, secretId string, versionId string) cacheVersion {
	return cacheVersion{
		versionId:   versionId,
		cacheObject: &cacheObject{config: config, client: client, secretId: secretId, refreshNeeded: true},
	}
}

// isRefreshNeeded determines if the cached item should be refreshed.
func (cv *cacheVersion) isRefreshNeeded() bool {
	return cv.cacheObject.isRefreshNeeded()
}

// refresh the cached object when needed.
func (cv *cacheVersion) refresh(ctx context.Context) {
	if !cv.isRefreshNeeded() {
		return
	}

	cv.refreshNeeded = false

	result, err := cv.executeRefresh(ctx)

	if err != nil {
		cv.errorCount++
		cv.err = err
		delay := exceptionRetryDelayBase * math.Pow(exceptionRetryGrowthFactor, float64(cv.errorCount))
		delay = math.Min(delay, exceptionRetryDelayMax)
		delayDuration := time.Nanosecond * time.Duration(delay)
		cv.nextRetryTime = time.Now().Add(delayDuration).UnixNano()
		return
	}

	cv.setWithHook(result)
	cv.err = nil
	cv.errorCount = 0

}

// executeRefresh performs the actual refresh of the cached secret information.
// Returns the GetSecretValue API result and an error if operation fails.
func (cv *cacheVersion) executeRefresh(ctx context.Context) (*secretsmanager.GetSecretValueOutput, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId:  &cv.secretId,
		VersionId: &cv.versionId,
	}
	return cv.client.GetSecretValue(ctx, input)
}

// getSecretValue gets the cached secret version value.
// Returns the GetSecretValue API cached result and an error if operation fails.
func (cv *cacheVersion) getSecretValue(ctx context.Context) (*secretsmanager.GetSecretValueOutput, error) {
	cv.mux.Lock()
	defer cv.mux.Unlock()

	cv.refresh(ctx)

	return cv.getWithHook(), cv.err
}

// setWithHook sets the cache item's data using the CacheHook, if one is configured.
func (cv *cacheVersion) setWithHook(result *secretsmanager.GetSecretValueOutput) {
	if cv.config.Hook != nil {
		cv.data = cv.config.Hook.Put(result)
	} else {
		cv.data = result
	}
}

// getWithHook gets the cache item's data using the CacheHook, if one is configured.
func (cv *cacheVersion) getWithHook() *secretsmanager.GetSecretValueOutput {
	var result interface{}
	if cv.config.Hook != nil {
		result = cv.config.Hook.Get(cv.data)
	} else {
		result = cv.data
	}

	if result == nil {
		return nil
	} else {
		return result.(*secretsmanager.GetSecretValueOutput)
	}
}
