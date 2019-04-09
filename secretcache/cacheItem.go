package secretcache

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

// secretCacheItem maintains a cache of secret versions.
type secretCacheItem struct {
	versions *lruCache

	// The next scheduled refresh time for this item.  Once the item is accessed
	// after this time, the item will be synchronously refreshed.
	nextRefreshTime int64
	*cacheObject
}

// newSecretCacheItem initialises a secretCacheItem using default cache size and sets next refresh time to now
func newSecretCacheItem(config CacheConfig, client secretsmanageriface.SecretsManagerAPI, secretId string) secretCacheItem {
	return secretCacheItem{
		versions:        newLRUCache(10),
		cacheObject:     &cacheObject{config: config, client: client, secretId: secretId, refreshNeeded: true},
		nextRefreshTime: time.Now().UnixNano(),
	}
}

// isRefreshNeeded determines if the cached item should be refreshed.
func (ci *secretCacheItem) isRefreshNeeded() bool {
	if ci.cacheObject.isRefreshNeeded() {
		return true
	}

	return ci.nextRefreshTime <= time.Now().UnixNano()
}

// getVersionId gets the version id for the given version stage.
// Returns the version id and a boolean to indicate success.
func (ci *secretCacheItem) getVersionId(versionStage string) (string, bool) {
	result := ci.getWithHook()
	if result == nil {
		return "", false
	}

	if result.VersionIdsToStages == nil {
		return "", false
	}

	for versionId, stages := range result.VersionIdsToStages {
		for _, stage := range stages {
			if versionStage == *stage {
				return versionId, true
			}
		}
	}

	return "", false
}

// executeRefresh performs the actual refresh of the cached secret information.
// Returns the DescribeSecret API result and an error if call failed.
func (ci *secretCacheItem) executeRefresh() (*secretsmanager.DescribeSecretOutput, error) {
	input := &secretsmanager.DescribeSecretInput{
		SecretId: &ci.secretId,
	}

	result, err := ci.client.DescribeSecretWithContext(aws.BackgroundContext(), input, request.WithAppendUserAgent(userAgent()))

	var maxTTL int64
	if ci.config.CacheItemTTL == 0 {
		maxTTL = DefaultCacheItemTTL
	} else {
		maxTTL = ci.config.CacheItemTTL
	}

	var ttl int64
	if maxTTL < 0 {
		return nil, &InvalidConfigError{
			baseError{
				Message: "cannot set negative ttl on cache",
			},
		}
	} else if maxTTL < 2 {
		ttl = maxTTL
	} else {
		ttl = rand.Int63n(maxTTL/2) + maxTTL/2
	}

	ci.nextRefreshTime = time.Now().Add(time.Nanosecond * time.Duration(ttl)).UnixNano()
	return result, err
}

// getVersion gets the secret cache version associated with the given stage.
// Returns a boolean to indicate operation success.
func (ci *secretCacheItem) getVersion(versionStage string) (*cacheVersion, bool) {
	versionId, versionIdFound := ci.getVersionId(versionStage)
	if !versionIdFound {
		return nil, false
	}

	cachedValue, cachedValueFound := ci.versions.get(versionId)

	if !cachedValueFound {
		cacheVersion := newCacheVersion(ci.config, ci.client, ci.secretId, versionId)
		ci.versions.putIfAbsent(versionId, &cacheVersion)
		cachedValue, _ = ci.versions.get(versionId)
	}

	secretCacheVersion, _ := cachedValue.(*cacheVersion)
	return secretCacheVersion, true
}

// refresh the cached object when needed.
func (ci *secretCacheItem) refresh() {
	if !ci.isRefreshNeeded() {
		return
	}

	ci.refreshNeeded = false

	result, err := ci.executeRefresh()

	if err != nil {
		ci.errorCount++
		ci.err = err
		delay := exceptionRetryDelayBase * math.Pow(exceptionRetryGrowthFactor, float64(ci.errorCount))
		delay = math.Min(delay, exceptionRetryDelayMax)
		delayDuration := time.Nanosecond * time.Duration(delay)
		ci.nextRetryTime = time.Now().Add(delayDuration).UnixNano()
		return
	}

	ci.setWithHook(result)
	ci.err = nil
	ci.errorCount = 0
}

// getSecretValue gets the cached secret value for the given version stage.
// Returns the GetSecretValue API result and an error if operation fails.
func (ci *secretCacheItem) getSecretValue(versionStage string) (*secretsmanager.GetSecretValueOutput, error) {
	if versionStage == "" && ci.config.VersionStage == "" {
		versionStage = DefaultVersionStage
	} else if versionStage == "" && ci.config.VersionStage != "" {
		versionStage = ci.config.VersionStage
	}

	ci.mux.Lock()
	defer ci.mux.Unlock()

	ci.refresh()
	version, ok := ci.getVersion(versionStage)

	if !ok {
		if ci.err != nil {
			return nil, ci.err
		} else {
			return nil, &VersionNotFoundError{
				baseError{
					Message: fmt.Sprintf("could not find secret version for versionStage %s", versionStage),
				},
			}
		}

	}
	return version.getSecretValue()
}

// setWithHook sets the cache item's data using the CacheHook, if one is configured.
func (ci *secretCacheItem) setWithHook(result *secretsmanager.DescribeSecretOutput) {
	if ci.config.Hook != nil {
		ci.data = ci.config.Hook.Put(result)
	} else {
		ci.data = result
	}
}

// getWithHook gets the cache item's data using the CacheHook, if one is configured.
func (ci *secretCacheItem) getWithHook() *secretsmanager.DescribeSecretOutput {
	var result interface{}
	if ci.config.Hook != nil {
		result = ci.config.Hook.Get(ci.data)
	} else {
		result = ci.data
	}

	if result == nil {
		return nil
	} else {
		return result.(*secretsmanager.DescribeSecretOutput)
	}
}
