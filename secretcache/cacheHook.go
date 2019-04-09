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

// CacheHook is an interface to hook into the local in-memory cache. This interface will allow
// users to perform actions on the items being stored in the in-memory
// cache. One example would be encrypting/decrypting items stored in the
// in-memory cache.
type CacheHook interface {

	// Put prepares the object for storing in the cache.
	Put(data interface{}) interface{}

	// Get derives the object from the cached object.
	Get(data interface{}) interface{}
}
