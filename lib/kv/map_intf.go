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
	AddOrUpdate(key K, obj V) error
	Get(key K) (item V, exists bool)
	Delete(key K) (V, error)
	Replace(items map[K]V) error
	ListKeys(filters ...SafeStoreKeyFilterFunc[K]) []K
	ListValues(keys ...K) (items []V)
}

type Map[K comparable, V any] interface {
	Put(key K, val V) error
	Get(key K) (item V, exists bool)
	Delete(key K) (V, error)
	Foreach(action func(i uint64, key K, val V) (_continue bool))
	MigrateFrom(m map[K]V) error
	Clear()
	Len() int64
}
