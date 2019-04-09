package secretcache

const (
	DefaultMaxCacheSize = 1024
	DefaultCacheItemTTL = 3600000000000 // 1 hour in nanoseconds
	DefaultVersionStage = "AWSCURRENT"
)

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
