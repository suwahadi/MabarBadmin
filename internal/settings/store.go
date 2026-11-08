package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
)

type Values map[string]json.RawMessage

type Store struct {
	q     *sqlc.Queries
	mu    sync.RWMutex
	cache Values
}

func NewStore(q *sqlc.Queries) *Store {
	return &Store{q: q}
}

// Get returns a snapshot of all settings, loading them once and serving later calls from memory.
func (s *Store) Get(ctx context.Context) (Values, error) {
	s.mu.RLock()
	c := s.cache
	s.mu.RUnlock()
	if c != nil {
		return c, nil
	}
	rows, err := s.q.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	v := Values{}
	for _, r := range rows {
		v[r.Key] = r.Value
	}
	s.mu.Lock()
	s.cache = v
	s.mu.Unlock()
	return v, nil
}

func (s *Store) Invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// Save writes the given typed values and drops the cache so the next request sees them.
func (s *Store) Save(ctx context.Context, q *sqlc.Queries, values map[string]any) error {
	for key, val := range values {
		if _, ok := Lookup(key); !ok {
			return fmt.Errorf("unknown setting %q", key)
		}
		raw, err := json.Marshal(val)
		if err != nil {
			return err
		}
		if err := q.UpsertSetting(ctx, sqlc.UpsertSettingParams{Key: key, Value: raw}); err != nil {
			return err
		}
	}
	s.Invalidate()
	return nil
}

// Seed inserts defaults for keys that do not exist yet and never overwrites admin changes.
func Seed(ctx context.Context, q *sqlc.Queries, overrides map[string]any) error {
	for _, d := range Registry {
		val := d.Default
		if o, ok := overrides[d.Key]; ok {
			val = o
		}
		if val == nil {
			val = zeroFor(d.Kind)
		}
		raw, err := json.Marshal(val)
		if err != nil {
			return err
		}
		if err := q.InsertSettingIfMissing(ctx, sqlc.InsertSettingIfMissingParams{Key: d.Key, Value: raw}); err != nil {
			return err
		}
	}
	return nil
}

func zeroFor(k Kind) any {
	switch k {
	case KindBool:
		return false
	case KindInt:
		return int64(0)
	case KindMedia:
		return nil
	default:
		return ""
	}
}

func (v Values) raw(key string) (json.RawMessage, bool) {
	r, ok := v[key]
	if !ok || len(r) == 0 || string(r) == "null" {
		return nil, false
	}
	return r, true
}

func (v Values) String(key string) string {
	if r, ok := v.raw(key); ok {
		var s string
		if json.Unmarshal(r, &s) == nil {
			return s
		}
	}
	if d, ok := Lookup(key); ok {
		if s, ok := d.Default.(string); ok {
			return s
		}
	}
	return ""
}

func (v Values) Bool(key string) bool {
	if r, ok := v.raw(key); ok {
		var b bool
		if json.Unmarshal(r, &b) == nil {
			return b
		}
	}
	if d, ok := Lookup(key); ok {
		if b, ok := d.Default.(bool); ok {
			return b
		}
	}
	return false
}

func (v Values) Int(key string) int64 {
	if r, ok := v.raw(key); ok {
		var n int64
		if json.Unmarshal(r, &n) == nil {
			return n
		}
	}
	if d, ok := Lookup(key); ok {
		if n, ok := d.Default.(int64); ok {
			return n
		}
	}
	return 0
}

func (v Values) MediaID(key string) *int64 {
	if r, ok := v.raw(key); ok {
		var n int64
		if json.Unmarshal(r, &n) == nil && n > 0 {
			return &n
		}
	}
	return nil
}

func (v Values) Location() *time.Location {
	if loc, err := time.LoadLocation(v.String(TimeZone)); err == nil {
		return loc
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	return loc
}

// MediaKeys lists setting keys that hold media ids, so the orphan sweeper and
// replacement logic know which media rows settings still reference.
func MediaKeys() []string {
	var keys []string
	for _, d := range Registry {
		if d.Kind == KindMedia {
			keys = append(keys, d.Key)
		}
	}
	return keys
}
