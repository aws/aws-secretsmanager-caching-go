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
