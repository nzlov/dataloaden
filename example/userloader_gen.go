// Code generated by github.com/vektah/dataloaden, DO NOT EDIT.

package example

import (
	"sync"
	"time"
)

// UserLoaderConfig captures the config to create a new UserLoader
type UserLoaderConfig struct {
	// Fetch is a method that provides the data for the loader
	Fetch func(keys []string) ([]*User, []error)

	// Wait is how long wait before sending a batch
	Wait time.Duration

	// Cache Time
	CacheTime time.Duration

	// MaxBatch will limit the maximum number of keys to send in one batch, 0 = not limit
	MaxBatch int
}

// NewUserLoader creates a new UserLoader given a fetch, wait, and maxBatch
func NewUserLoader(config UserLoaderConfig) *UserLoader {
	return &UserLoader{
		fetch:     config.Fetch,
		wait:      config.Wait,
		maxBatch:  config.MaxBatch,
		cachetime: config.CacheTime,
		cache:     &sync.Map{},
	}
}

type UserLoaderCacheItem struct {
	last time.Time
	v    *User
}

// UserLoader batches and caches requests
type UserLoader struct {
	// this method provides the data for the loader
	fetch func(keys []string) ([]*User, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// INTERNAL

	// lazily created cache
	//cache map[string]*UserLoaderCacheItem
	cache *sync.Map
	// cache timeout
	cachetime time.Duration

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *userLoaderBatch

	// mutex to prevent races
	mu sync.Mutex
}

type userLoaderBatch struct {
	keys    []string
	data    []*User
	error   []error
	closing bool
	done    chan struct{}
}

// Load a User by key, batching and caching will be applied automatically
func (l *UserLoader) Load(key string) (*User, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a User.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *UserLoader) LoadThunk(key string) func() (*User, error) {
	if it, ok := l.cache.Load(key); ok {
		return func() (*User, error) {
			iv := it.(*UserLoaderCacheItem)
			iv.last = time.Now()
			return iv.v, nil
		}
	}

	if l.batch == nil {
		l.batch = &userLoaderBatch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)

	return func() (*User, error) {
		<-batch.done

		var data *User
		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		// its convenient to be able to return a single error for everything
		if len(batch.error) == 1 {
			err = batch.error[0]
		} else if batch.error != nil {
			err = batch.error[pos]
		}

		if err == nil {
			l.unsafeSet(key, data)
		}

		return data, err
	}
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *UserLoader) LoadAll(keys []string) ([]*User, []error) {
	results := make([]func() (*User, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	users := make([]*User, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		users[i], errors[i] = thunk()
	}
	return users, errors
}

// LoadAllThunk returns a function that when called will block waiting for a Users.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *UserLoader) LoadAllThunk(keys []string) func() ([]*User, []error) {
	results := make([]func() (*User, error), len(keys))
	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]*User, []error) {
		users := make([]*User, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			users[i], errors[i] = thunk()
		}
		return users, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *UserLoader) Prime(key string, value *User) bool {
	var found bool
	if _, found = l.cache.Load(key); !found {
		// make a copy when writing to the cache, its easy to pass a pointer in from a loop var
		// and end up with the whole cache pointing to the same value.
		cpy := *value
		l.unsafeSet(key, &cpy)
	}
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *UserLoader) Clear(key string) {
	l.cache.Delete(key)
}

func (l *UserLoader) unsafeSet(key string, value *User) {
	l.cache.Store(key, &UserLoaderCacheItem{
		last: time.Now(),
		v:    value,
	})
}

// CacheRotation Rotating cache time
func (l *UserLoader) CacheRotation(t time.Time) {
	l.cache.Range(func(k, v interface{}) bool {
		iv := v.(*UserLoaderCacheItem)
		if t.Sub(iv.last) > l.cachetime {
			l.cache.Delete(k)
		}
		iv.last = t
		return true
	})
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *userLoaderBatch) keyIndex(l *UserLoader, key string) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go b.startTimer(l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go b.end(l)
		}
	}

	return pos
}

func (b *userLoaderBatch) startTimer(l *UserLoader) {
	time.Sleep(l.wait)

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		return
	}

	l.batch = nil

	b.end(l)
}

func (b *userLoaderBatch) end(l *UserLoader) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}
