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


type baseError struct {
	Message string
}

type VersionNotFoundError struct {
	baseError
}

func (v *VersionNotFoundError) Error() string {
	return v.Message
}

type InvalidConfigError struct {
	baseError
}

func (i *InvalidConfigError) Error() string {
	return i.Message
}

type InvalidOperationError struct {
	baseError
}

func (i *InvalidOperationError) Error() string {
	return i.Message
}

