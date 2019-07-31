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
	"strconv"
	"testing"
)

func TestPutIfAbsent(t *testing.T) {

	lruCache := newLRUCache(DefaultMaxCacheSize)
	key := "some-key"
	data := 42
	addedToCache := lruCache.putIfAbsent(key, data)

	if !addedToCache {
		t.Fatalf("Failed initial add to cache")
	}

	addedToCache = lruCache.putIfAbsent(key, data*2)

	if addedToCache {
		t.Fatalf("Should have failed second add to cache")
	}

	retrievedItem, found := lruCache.cacheMap[key]

	if !found {
		t.Fatalf("Did not find expected entry in cache")
	}

	if (*retrievedItem).data != data {
		t.Fatalf("Expected data %d did not match retrieved data %d", data, (*retrievedItem).data)
	}

	if lruCache.cacheSize != 1 {
		t.Fatalf("Expected cache size to be 1")
	}
}

func TestGet(t *testing.T) {
	lruCache := newLRUCache(DefaultMaxCacheSize)
	key := "some-key"
	data := 42

	_, found := lruCache.get(key)

	if found {
		t.Fatalf("Did not expect entry in cache")
	}

	lruCache.putIfAbsent(key, data)

	retrievedData, found := lruCache.get(key)

	if !found {
		t.Fatalf("Did not find expected entry in cache")
	}

	if retrievedData != data {
		t.Fatalf("Expected data %d did not match retrieved data %d", data, retrievedData)
	}
}

func TestLRUCacheMax(t *testing.T) {
	lruCache := newLRUCache(10)
	for i := 0; i <= 100; i++ {
		lruCache.putIfAbsent(strconv.Itoa(i), i)
	}

	for i := 0; i <= 90; i++ {
		if _, found := lruCache.get(strconv.Itoa(i)); found {
			t.Fatalf("Found unexpected val in cache - %d", i)
		}
	}

	for i := 91; i <= 100; i++ {
		if val, found := lruCache.get(strconv.Itoa(i)); !found || i != val.(int) {
			t.Fatalf("Expected to find val in cache - %d", i)
		}
	}
}

func TestLRUCacheEmpty(t *testing.T) {
	lruCache := newLRUCache(10)
	_, found := lruCache.get("some-key")

	if found {
		t.Fatalf("Found unexpected val in cache")
	}
}

func TestLRUCacheRecent(t *testing.T) {
	lruCache := newLRUCache(10)
	for i := 0; i <= 100; i++ {
		lruCache.putIfAbsent(strconv.Itoa(i), i)
		lruCache.get("0")
	}

	for i := 1; i <= 91; i++ {
		if _, found := lruCache.get(strconv.Itoa(i)); found {
			t.Fatalf("Found unexpected val in cache - %d", i)
		}
	}

	for i := 92; i <= 100; i++ {
		if val, found := lruCache.get(strconv.Itoa(i)); !found || i != val.(int) {
			t.Fatalf("Expected to find val in cache - %d", i)
		}
	}

	if val, found := lruCache.get("0"); !found || 0 != val.(int) {
		t.Fatalf("Expected to find val in cache - %d", 0)
	}
}

func TestLRUCacheZero(t *testing.T) {
	lruCache := newLRUCache(0)

	for i := 0; i <= 100; i++ {
		strI := strconv.Itoa(i)
		lruCache.putIfAbsent(strI, i)
		if _, found := lruCache.get(strI); found {
			t.Fatalf("Found unexpected val in cache - %d", i)
		}
	}

	for i := 0; i <= 100; i++ {
		if _, found := lruCache.get(strconv.Itoa(i)); found {
			t.Fatalf("Found unexpected val in cache - %d", i)
		}
	}
}

func TestLRUCacheOne(t *testing.T) {
	lruCache := newLRUCache(1)

	for i := 0; i <= 100; i++ {
		strI := strconv.Itoa(i)
		lruCache.putIfAbsent(strI, i)
		if val, found := lruCache.get(strI); !found || i != val.(int) {
			t.Fatalf("Expected to find val in cache - %d", i)
		}
	}

	for i := 0; i <= 99; i++ {
		if _, found := lruCache.get(strconv.Itoa(i)); found {
			t.Fatalf("Found unexpected val in cache - %d", i)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := newLRUCache(1)
	cache.putIfAbsent("key", "value")

	success := make(chan bool)

	go accessor(cache, 500, "key", "value", success)
	go accessor(cache, 400, "key", "value", success)
	go accessor(cache, 300, "key", "value", success)
	go accessor(cache, 600, "key", "value", success)

	for i := 0; i < 4; i++ {
		if !<-success {
			t.Fatalf("Expected value not found")
		}
	}
}

func accessor(cache *lruCache, n int, key string, value string, success chan bool) {
	for i := 0; i < n; i++ {
		if val, found := cache.get(key); !found || val != value {
			success <- false
		}
	}

	success <- true
}

func TestConcurrentMutations(t *testing.T) {
	cache := newLRUCache(1)
	done := make(chan bool)

	go mutator(cache, 500, "key", done)

	go func(n int, key string, done chan bool) {
		for i := 0; i < n; i++ {
			cache.get(key)
		}
		done <- true
	}(2000, "key", done)

	go mutator(cache, 400, "key", done)

	for i := 0; i < 3; i++ {
		<-done
	}

	if _, found := cache.get("key"); !found {
		t.Fatalf("Expected to find a value in cache after concurrent mutations")
	}
}

func mutator(cache *lruCache, n int, key string, done chan bool) {
	for i := 0; i < n; i++ {
		cache.putIfAbsent(key, i)
	}
	done <- true
}
