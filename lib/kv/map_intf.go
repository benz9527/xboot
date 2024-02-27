package kv

import "io"

type SafeStoreKeyFilterFunc[K comparable] func(key K) bool

func defaultAllKeysFilter[K comparable](key K) bool {
	return true
}

type Closable interface {
	io.Closer
}

type ThreadSafeStorer[K comparable, V any] interface {
	Purge() error
	AddOrUpdate(key K, obj V)
	Replace(items map[K]V)
	Delete(key K)
	Get(key K) (item V, exists bool)
	ListKeys(filters ...SafeStoreKeyFilterFunc[K]) []K
	ListValues(keys ...K) (items []V)
}
