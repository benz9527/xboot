package kv

import (
	"io"
	"log/slog"
	"reflect"
	"sync"
)

type threadSafeMap[K comparable, V any] struct {
	lock           sync.RWMutex
	m              *SwissMap[K, V]
	isClosableItem bool
}

func (t *threadSafeMap[K, V]) AddOrUpdate(key K, obj V) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.m.Put(key, obj)
}

func (t *threadSafeMap[K, V]) Replace(items map[K]V) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.m.Clear()
	return t.m.MigrateFrom(items)
}

func (t *threadSafeMap[K, V]) Delete(key K) (V, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.m.Delete(key)
}

func (t *threadSafeMap[K, V]) Get(key K) (item V, exists bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	item, exists = t.m.Get(key)
	return
}

func (t *threadSafeMap[K, V]) ListKeys(filters ...SafeStoreKeyFilterFunc[K]) []K {

	realFilters := make([]SafeStoreKeyFilterFunc[K], 0, len(filters))
	for _, filter := range filters {
		if filter != nil {
			realFilters = append(realFilters, filter)
		}
	}
	if len(realFilters) == 0 {
		realFilters = append(realFilters, defaultAllKeysFilter[K])
	}

	t.lock.RLock()
	defer t.lock.RUnlock()

	keys := make([]K, 0, t.m.Len())
	t.m.Foreach(func(i uint64, key K, val V) bool {
		for _, filter := range realFilters {
			if filter(key) {
				keys = append(keys, key)
				break
			}
		}
		return true
	})
	return keys
}

func (t *threadSafeMap[K, V]) ListValues(keys ...K) (items []V) {
	realKeys := make([]K, 0, len(keys))
	for _, key := range keys {
		realKeys = append(realKeys, key)
	}

	contains := func(keys []K, key K) bool {
		for _, k := range keys {
			if k == key {
				return true
			}
		}
		return false
	}

	t.lock.RLock()
	defer t.lock.RUnlock()
	values := make([]V, 0, t.m.Len())
	t.m.Foreach(func(i uint64, key K, val V) bool {
		if len(realKeys) > 0 && contains(realKeys, key) {
			values = append(values, val)
		} else if len(realKeys) == 0 {
			values = append(values, val)
		}
		return true
	})
	return values
}

func (t *threadSafeMap[K, V]) Purge() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.isClosableItem {
		t.m.Foreach(func(i uint64, key K, val V) bool {
			if reflect.ValueOf(val).IsNil() {
				return true
			}
			typ := reflect.TypeOf(val)
			if typ.Implements(reflect.TypeOf((*io.Closer)(nil)).Elem()) {
				vals := reflect.ValueOf(val).MethodByName("Close").Call([]reflect.Value{})
				if len(vals) > 0 && !vals[0].IsNil() {
					intf := vals[0].Elem().Interface()
					switch intf.(type) {
					case error:
						err := intf.(error)
						slog.Error("Purge info", "error", err)
					}
				}
			}
			return true
		})
	}
	t.m = nil
	return nil
}

func NewThreadSafeMap[K comparable, V any]() ThreadSafeStorer[K, V] {
	isCloserItem := false
	nilT := new(V)
	if !reflect.ValueOf(nilT).IsNil() {
		if reflect.TypeOf(nilT).Implements(reflect.TypeOf((*io.Closer)(nil)).Elem()) {
			isCloserItem = true
		}
	} else {
		_nilT := *new(V)
		if reflect.TypeOf(_nilT).Implements(reflect.TypeOf((*io.Closer)(nil)).Elem()) {
			isCloserItem = true
		}
	}

	return &threadSafeMap[K, V]{m: NewSwissMap[K, V](1024), isClosableItem: isCloserItem}
}
