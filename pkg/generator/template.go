package generator

import "text/template"

var tpl = template.Must(template.New("generated").
	Funcs(template.FuncMap{
		"lcFirst": lcFirst,
	}).
	Parse(`
// Code generated by github.com/vektah/dataloaden, DO NOT EDIT.

package {{.Package}}

import (
    "sync"
    "time"

    {{if .KeyType.ImportPath}}"{{.KeyType.ImportPath}}"{{end}}
    {{if .ValType.ImportPath}}"{{.ValType.ImportPath}}"{{end}}
)

// {{.Name}}Config captures the config to create a new {{.Name}}
type {{.Name}}Config struct {
	// Fetch is a method that provides the data for the loader 
	Fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// Wait is how long wait before sending a batch
	Wait time.Duration

	// Cache Time
	CacheTime time.Duration

	// MaxBatch will limit the maximum number of keys to send in one batch, 0 = not limit
	MaxBatch int
}

// New{{.Name}} creates a new {{.Name}} given a fetch, wait, and maxBatch
func New{{.Name}}(config {{.Name}}Config) *{{.Name}} {
	return &{{.Name}}{
		fetch: config.Fetch,
		wait: config.Wait,
		maxBatch: config.MaxBatch,
        cachetime: config.CacheTime,
        cache: &sync.Map{},
	}
}

type {{.Name}}CacheItem struct{
    last time.Time
    v {{.ValType.String}}
}

// {{.Name}} batches and caches requests          
type {{.Name}} struct {
	// this method provides the data for the loader
	fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// INTERNAL

	// lazily created cache
	//cache map[{{.KeyType.String}}]*{{.Name}}CacheItem
    cache *sync.Map
    // cache timeout
    cachetime time.Duration

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *{{.Name|lcFirst}}Batch

	// mutex to prevent races
	mu sync.Mutex
}

type {{.Name|lcFirst}}Batch struct {
	keys    []{{.KeyType}}
	data    []{{.ValType.String}}
	error   []error
	closing bool
	done    chan struct{}
}

// Load a {{.ValType.Name}} by key, batching and caching will be applied automatically
func (l *{{.Name}}) Load(key {{.KeyType.String}}) ({{.ValType.String}}, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a {{.ValType.Name}}.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadThunk(key {{.KeyType.String}}) func() ({{.ValType.String}}, error) {
	if it, ok := l.cache.Load(key); ok {
	   return func() ({{.ValType.String}}, error) {
           iv := it.(*{{.Name}}CacheItem)
           iv.last = time.Now()
	       return iv.v, nil
	   }
	}

	if l.batch == nil {
		l.batch = &{{.Name|lcFirst}}Batch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)

	return func() ({{.ValType.String}}, error) {
		<-batch.done

		var data {{.ValType.String}}
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
func (l *{{.Name}}) LoadAll(keys []{{.KeyType}}) ([]{{.ValType.String}}, []error) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
	}
	return {{.ValType.Name|lcFirst}}s, errors
}

// LoadAllThunk returns a function that when called will block waiting for a {{.ValType.Name}}s.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadAllThunk(keys []{{.KeyType}}) (func() ([]{{.ValType.String}}, []error)) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))
 	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]{{.ValType.String}}, []error) {
		{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
		}
		return {{.ValType.Name|lcFirst}}s, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *{{.Name}}) Prime(key {{.KeyType}}, value {{.ValType.String}}) bool {
	var found bool
	if _, found = l.cache.Load(key); !found {
		{{- if .ValType.IsPtr }}
			// make a copy when writing to the cache, its easy to pass a pointer in from a loop var
			// and end up with the whole cache pointing to the same value.
			cpy := *value
			l.unsafeSet(key, &cpy)
		{{- else if .ValType.IsSlice }}
			// make a copy when writing to the cache, its easy to pass a pointer in from a loop var
			// and end up with the whole cache pointing to the same value.
			cpy := make({{.ValType.String}}, len(value))
			copy(cpy, value)
			l.unsafeSet(key, cpy)
		{{- else }}
			l.unsafeSet(key, value)
		{{- end }}
	}
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *{{.Name}}) Clear(key {{.KeyType}}) {
    l.cache.Delete(key)
}

func (l *{{.Name}}) unsafeSet(key {{.KeyType}}, value {{.ValType.String}}) {
    l.cache.Store(key,&{{.Name}}CacheItem{
        last: time.Now(),
        v:value,
    })
}

// CacheRotation Rotating cache time
func (l *{{.Name}}) CacheRotation(t time.Time) {
    l.cache.Range(func(k, v interface{}) bool{
        iv:=v.(*{{.Name}}CacheItem)
        if t.Sub(iv.last) > l.cachetime {
            l.cache.Delete(k)
        }
        iv.last = t
        return true
    })
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *{{.Name|lcFirst}}Batch) keyIndex(l *{{.Name}}, key {{.KeyType}}) int {
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

func (b *{{.Name|lcFirst}}Batch) startTimer(l *{{.Name}}) {
	time.Sleep(l.wait)

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		return
	}

	l.batch = nil

	b.end(l)
}

func (b *{{.Name|lcFirst}}Batch) end(l *{{.Name}}) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}
`))

var tplCache = template.Must(template.New("generatedcache").
	Funcs(template.FuncMap{
		"lcFirst": lcFirst,
		"pFirst":  pFirst,
	}).
	Parse(`
// Code generated by github.com/vektah/dataloaden, DO NOT EDIT.

package {{.Package}}

import (
    "encoding/json"
    "sync"
    "time"

    {{if .KeyType.ImportPath}}"{{.KeyType.ImportPath}}"{{end}}
    {{if .ValType.ImportPath}}"{{.ValType.ImportPath}}"{{end}}
)

type {{.Name|lcFirst}}Cache interface {
	Save(string, []byte)
	Get(string) ([]byte, bool)
	Clear(string)
}

func {{.Name|lcFirst}}Key(key {{.KeyType.String}}) string {
	return fmt.Sprintf("nzlov@Cache:{{.Name}}:%v", key)
}
func {{.Name|lcFirst}}WithBytes(value []byte) {{.ValType.String}} {
    if string(value) == ""{
	{{- if .ValType.IsPtr }}
    return nil
	{{- else if .ValType.IsSlice }}
    return nil
    {{- end }}
    }

	o := {{.ValType.String|pFirst}}{}
	json.Unmarshal(value, &o)
	{{- if .ValType.IsPtr }}
	return &o
    {{- else }}
	return o
    {{- end }}
}

// {{.Name}}Config captures the config to create a new {{.Name}}
type {{.Name}}Config struct {
	// Fetch is a method that provides the data for the loader 
	Fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// Wait is how long wait before sending a batch
	Wait time.Duration

	// MaxBatch will limit the maximum number of keys to send in one batch, 0 = not limit
	MaxBatch int

    Cache {{.Name|lcFirst}}Cache
}

// New{{.Name}} creates a new {{.Name}} given a fetch, wait, and maxBatch
func New{{.Name}}(config {{.Name}}Config) *{{.Name}} {
	return &{{.Name}}{
		fetch: config.Fetch,
		wait: config.Wait,
		maxBatch: config.MaxBatch,
        cache: config.Cache,
	}
}

type {{.Name}}CacheItem struct{
    last time.Time
    v {{.ValType.String}}
}

// {{.Name}} batches and caches requests          
type {{.Name}} struct {
	// this method provides the data for the loader
	fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

    cache {{.Name|lcFirst}}Cache

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *{{.Name|lcFirst}}Batch
}

type {{.Name|lcFirst}}Batch struct {
	keys    []{{.KeyType}}
	data    []{{.ValType.String}}
	error   []error
	closing bool
	done    chan struct{}
}

// Load a {{.ValType.Name}} by key, batching and caching will be applied automatically
func (l *{{.Name}}) Load(key {{.KeyType.String}}) ({{.ValType.String}}, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a {{.ValType.Name}}.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadThunk(key {{.KeyType.String}}) func() ({{.ValType.String}}, error) {
	if it, ok := l.cache.Get({{.Name|lcFirst}}Key(key)); ok {
	   return func() ({{.ValType.String}}, error) {
	    	return {{.Name|lcFirst}}WithBytes(it), nil
	    }
	}


	if l.batch == nil {
		l.batch = &{{.Name|lcFirst}}Batch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)

	return func() ({{.ValType.String}}, error) {
		<-batch.done

		var data {{.ValType.String}}
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
func (l *{{.Name}}) LoadAll(keys []{{.KeyType}}) ([]{{.ValType.String}}, []error) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
	}
	return {{.ValType.Name|lcFirst}}s, errors
}

// LoadAllThunk returns a function that when called will block waiting for a {{.ValType.Name}}s.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadAllThunk(keys []{{.KeyType}}) (func() ([]{{.ValType.String}}, []error)) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))
 	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]{{.ValType.String}}, []error) {
		{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
		}
		return {{.ValType.Name|lcFirst}}s, errors
	}
}

// Clear the value at key from the cache, if it exists
func (l *{{.Name}}) Clear(key {{.KeyType}}) {
	l.cache.Clear({{.Name|lcFirst}}Key(key))
}

func (l *{{.Name}}) unsafeSet(key {{.KeyType}}, value {{.ValType.String}}) {
    data := []byte{}
	{{- if .ValType.IsPtr }}
    if value != nil{
        data, _ = json.Marshal(value)
    }
	{{- else if .ValType.IsSlice }}
    if value != nil{
        data, _ = json.Marshal(value)
    }
	{{- else }}
        data, _ = json.Marshal(value)
	{{- end }}
	l.cache.Save({{.Name|lcFirst}}Key(key), data)
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *{{.Name|lcFirst}}Batch) keyIndex(l *{{.Name}}, key {{.KeyType}}) int {
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

func (b *{{.Name|lcFirst}}Batch) startTimer(l *{{.Name}}) {
	time.Sleep(l.wait)

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		return
	}

	l.batch = nil

	b.end(l)
}

func (b *{{.Name|lcFirst}}Batch) end(l *{{.Name}}) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}
`))

var tplExpire = template.Must(template.New("generatedexpire").
	Funcs(template.FuncMap{
		"lcFirst": lcFirst,
		"pFirst":  pFirst,
	}).
	Parse(`
// Code generated by github.com/vektah/dataloaden, DO NOT EDIT.

package {{.Package}}

import (
    "encoding/json"
    "sync"
    "time"

    {{if .KeyType.ImportPath}}"{{.KeyType.ImportPath}}"{{end}}
    {{if .ValType.ImportPath}}"{{.ValType.ImportPath}}"{{end}}
)

type {{.Name|lcFirst}}Cache interface {
	SaveExpire(string, time.Duration, []byte)
	GetExpire(string,time.Duration) ([]byte, bool)
	Clear(string)
}

func {{.Name|lcFirst}}Key(key {{.KeyType.String}}) string {
	return fmt.Sprintf("nzlov@Cache:{{.Name}}:%v", key)
}
func {{.Name|lcFirst}}WithBytes(value []byte) {{.ValType.String}} {
    if string(value) == ""{
	{{- if .ValType.IsPtr }}
    return nil
	{{- else if .ValType.IsSlice }}
    return nil
    {{- end }}
    }

	o := {{.ValType.String|pFirst}}{}
	json.Unmarshal(value, &o)
	{{- if .ValType.IsPtr }}
	return &o
    {{- else }}
	return o
    {{- end }}
}

// {{.Name}}Config captures the config to create a new {{.Name}}
type {{.Name}}Config struct {
	// Fetch is a method that provides the data for the loader 
	Fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// Wait is how long wait before sending a batch
	Wait time.Duration

	// MaxBatch will limit the maximum number of keys to send in one batch, 0 = not limit
	MaxBatch int

    Cache {{.Name|lcFirst}}Cache
    CacheTime time.Duration
}

// New{{.Name}} creates a new {{.Name}} given a fetch, wait, and maxBatch
func New{{.Name}}(config {{.Name}}Config) *{{.Name}} {
	return &{{.Name}}{
		fetch: config.Fetch,
		wait: config.Wait,
		maxBatch: config.MaxBatch,
        cache: config.Cache,
        cacheTime: config.CacheTime,
	}
}

type {{.Name}}CacheItem struct{
    last time.Time
    v {{.ValType.String}}
}

// {{.Name}} batches and caches requests          
type {{.Name}} struct {
	// this method provides the data for the loader
	fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

    cache {{.Name|lcFirst}}Cache
	cacheTime time.Duration

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *{{.Name|lcFirst}}Batch
}

type {{.Name|lcFirst}}Batch struct {
	keys    []{{.KeyType}}
	data    []{{.ValType.String}}
	error   []error
	closing bool
	done    chan struct{}
}

// Load a {{.ValType.Name}} by key, batching and caching will be applied automatically
func (l *{{.Name}}) Load(key {{.KeyType.String}}) ({{.ValType.String}}, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a {{.ValType.Name}}.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadThunk(key {{.KeyType.String}}) func() ({{.ValType.String}}, error) {
    if it, ok := l.cache.GetExpire({{.Name|lcFirst}}Key(key),l.cacheTime); ok {
	    return func() ({{.ValType.String}}, error) {
	    	return {{.Name|lcFirst}}WithBytes(it), nil
	    }
	}

	if l.batch == nil {
		l.batch = &{{.Name|lcFirst}}Batch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)

	return func() ({{.ValType.String}}, error) {
		<-batch.done

		var data {{.ValType.String}}
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
func (l *{{.Name}}) LoadAll(keys []{{.KeyType}}) ([]{{.ValType.String}}, []error) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
	}
	return {{.ValType.Name|lcFirst}}s, errors
}

// LoadAllThunk returns a function that when called will block waiting for a {{.ValType.Name}}s.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadAllThunk(keys []{{.KeyType}}) (func() ([]{{.ValType.String}}, []error)) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))
 	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]{{.ValType.String}}, []error) {
		{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
		}
		return {{.ValType.Name|lcFirst}}s, errors
	}
}

// Clear the value at key from the cache, if it exists
func (l *{{.Name}}) Clear(key {{.KeyType}}) {
	l.cache.Clear({{.Name|lcFirst}}Key(key))
}

func (l *{{.Name}}) unsafeSet(key {{.KeyType}}, value {{.ValType.String}}) {
    data := []byte{}
	{{- if .ValType.IsPtr }}
    if value != nil{
        data, _ = json.Marshal(value)
    }
	{{- else if .ValType.IsSlice }}
    if value != nil{
        data, _ = json.Marshal(value)
    }
	{{- else }}
        data, _ = json.Marshal(value)
	{{- end }}
	l.cache.SaveExpire({{.Name|lcFirst}}Key(key), l.cacheTime, data)
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *{{.Name|lcFirst}}Batch) keyIndex(l *{{.Name}}, key {{.KeyType}}) int {
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

func (b *{{.Name|lcFirst}}Batch) startTimer(l *{{.Name}}) {
	time.Sleep(l.wait)

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		return
	}

	l.batch = nil

	b.end(l)
}

func (b *{{.Name|lcFirst}}Batch) end(l *{{.Name}}) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}
`))
