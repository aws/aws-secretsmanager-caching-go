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

// Package secretcache provides the Cache struct for in-memory caching of secrets stored in AWS Secrets Manager
// Also exports a CacheHook, for pre-store and post-fetch processing of cached values
package secretcache

const (
	DefaultMaxCacheSize = 1024
	DefaultCacheItemTTL = 3600000000000 // 1 hour in nanoseconds
	DefaultVersionStage = "AWSCURRENT"
)

// CacheConfig is the config object passed to the Cache struct
type CacheConfig struct {

	//The maximum number of cached secrets to maintain before evicting secrets that
	//have not been accessed recently.
	MaxCacheSize int

	//The number of nanoseconds that a cached item is considered valid before
	// requiring a refresh of the secret state.  Items that have exceeded this
	// TTL will be refreshed synchronously when requesting the secret value.  If
	// the synchronous refresh failed, the stale secret will be returned.
	CacheItemTTL int64

	//The version stage that will be used when requesting the secret values for
	//this cache.
	VersionStage string

	//Used to hook in-memory cache updates.
	Hook CacheHook
}
