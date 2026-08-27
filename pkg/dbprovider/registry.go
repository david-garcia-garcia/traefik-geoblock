package dbprovider

import "sync"

// Registry stores one value per config hash. Traefik calls New on every reload;
// the same config must reuse the instance so a second 24h ticker is not started.
type Registry struct {
	mu sync.RWMutex
	m  map[string]any
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{m: make(map[string]any)}
}

// GetOrCreate returns the existing value for key, or stores the result of create.
func (r *Registry) GetOrCreate(key string, create func() (any, error)) (any, error) {
	r.mu.RLock()
	if v, ok := r.m[key]; ok {
		r.mu.RUnlock()
		return v, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.m[key]; ok {
		return v, nil
	}
	v, err := create()
	if err != nil {
		return nil, err
	}
	r.m[key] = v
	return v, nil
}

// Clear calls each on every stored value, then empties the map.
func (r *Registry) Clear(each func(any)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range r.m {
		if each != nil {
			each(v)
		}
		delete(r.m, k)
	}
}
