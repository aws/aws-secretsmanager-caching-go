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

	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
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
	client        secretsmanageriface.SecretsManagerAPI
	secretId      string
	err           error
	errorCount    int
	refreshNeeded bool

	// The time to wait before retrying a failed AWS Secrets Manager request.
	nextRetryTime int64
	data          interface{}
}

// isRefreshNeeded determines if the cached object should be refreshed.
func (o *cacheObject) isRefreshNeeded() bool {
	if o.refreshNeeded {
		return true
	}

	if o.err == nil {
		return false
	}

	if o.nextRetryTime == 0 {
		return true
	}

	return o.nextRetryTime <= time.Now().UnixNano()
}
