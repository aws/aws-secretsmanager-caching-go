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
)

// lruCache is a cache implementation using a map and doubly linked list.
type lruCache struct {
	cacheMap     map[string]*lruItem
	cacheMaxSize int
	cacheSize    int
	mux          sync.Mutex
	head         *lruItem
	tail         *lruItem
}

// lruItem is the cache item to hold data and linked list pointers.
type lruItem struct {
	next *lruItem
	prev *lruItem
	key  string
	data interface{}
}

// newLRUCache initialises an lruCache instance with given max size.
func newLRUCache(maxSize int) *lruCache {
	return &lruCache{
		cacheMap:     make(map[string]*lruItem),
		cacheMaxSize: maxSize,
	}
}

// get gets the cached item's data for the given key.
// Updates the fetched item to be head of the linked list.
func (l *lruCache) get(key string) (interface{}, bool) {
	l.mux.Lock()
	defer l.mux.Unlock()

	item, found := l.cacheMap[key]

	if !found {
		return nil, false
	}

	l.updateHead(item)

	return item.data, true
}

// putIfAbsent puts an lruItem initialised from the given data in the cache.
// Updates head of the linked list to be the new lruItem.
// If cache size is over max allowed size, removes the tail item from cache.
// Returns true if new key is inserted to cache, false if it already existed.
func (l *lruCache) putIfAbsent(key string, data interface{}) bool {
	l.mux.Lock()
	defer l.mux.Unlock()

	_, found := l.cacheMap[key]

	if found {
		return false
	}

	item := &lruItem{key: key, data: data}
	l.cacheMap[key] = item

	l.cacheSize++
	l.updateHead(item)

	if l.cacheSize > l.cacheMaxSize {
		delete(l.cacheMap, (*l.tail).key)
		l.unlink(l.tail)
		l.cacheSize--
	}

	return true
}

// updateHead updates head of the linked list to be the input lruItem.
func (l *lruCache) updateHead(item *lruItem) {
	if l.head == item {
		return
	}

	l.unlink(item)
	item.next = l.head

	if l.head != nil {
		l.head.prev = item
	}

	l.head = item

	if l.tail == nil {
		l.tail = item
	}
}

// unlink removes the input lruItem from the linked list.
func (l *lruCache) unlink(item *lruItem) {
	if l.head == item {
		l.head = item.next
	}

	if l.tail == item {
		l.tail = item.prev
	}

	if item.prev != nil {
		item.prev.next = item.next
	}

	if item.next != nil {
		item.next.prev = item.prev
	}

}
