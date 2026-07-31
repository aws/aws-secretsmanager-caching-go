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
	"sync"
	"time"
)

const (
	exceptionRetryDelayBase    = 1
	exceptionRetryGrowthFactor = 2
	exceptionRetryDelayMax     = 3600
	forceRefreshJitterSleep    = 5000
)

// Base cache object for common properties.
type cacheObject struct {
	mux           sync.Mutex
	config        CacheConfig
	client        SecretsManagerAPIClient
	secretId      string
	err           error
	errorCount    int
	refreshNeeded bool

	// The time to wait before retrying a failed AWS Secrets Manager request.
	nextRetryTime time.Time
	data          interface{}

	// now overrides time.Now in tests. nil in production.
	now func() time.Time
}

// Function used for overing the time.Now in tests. In production, it will
// just return the result of the normal time.Now function
func (o *cacheObject) timeNow() time.Time {
	if o.now != nil {
		return o.now()
	}
	return time.Now()
}

// isRefreshNeeded determines if the cached object should be refreshed.
func (o *cacheObject) isRefreshNeeded() bool {
	if o.refreshNeeded {
		return true
	}

	if o.err == nil {
		return false
	}

	if o.nextRetryTime.IsZero() {
		return true
	}

	// Compare both the monotonic and wall clock time to reduce possibility of secrets living longer than they should be
	// Note: During normal comparison, the monotonic clock is used. Round(0) will force the wall clock reading to be used.
	return o.nextRetryTime.Compare(o.timeNow()) <= 0 || o.nextRetryTime.Round(0).Compare(o.timeNow().Round(0)) <= 0
}
