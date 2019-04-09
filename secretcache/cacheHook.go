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
