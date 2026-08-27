// Package store provides the embedded persistent store for the shotcrete
// closure service. It is built on bbolt, a pure-Go transactional key/value
// database with no Cgo dependency, so a single binary serves both linux/amd64
// and linux/arm64. Every write command runs inside an ACID transaction and the
// database file is reopened on startup to recover leases, device calls,
// operation records and partial cycle state exactly as persisted.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

// Bucket names. They are stable identifiers and must not change across
// releases, otherwise existing databases would fail to open.
var (
	bucketCycles    = []byte("cycles")
	bucketOps       = []byte("operations")
	bucketLeases    = []byte("leases")
	bucketInventory = []byte("inventory")
	bucketDevices   = []byte("device_calls")
	bucketMeta      = []byte("meta")
)

// Store wraps a bbolt database and provides typed transactional access.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the database at path. The parent directory is created
// when missing. A lock file prevents two processes from opening the same file.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create directory: %w", err)
		}
	}
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// init creates every bucket and records the schema version for recovery.
func (s *Store) init() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{
			bucketCycles, bucketOps, bucketLeases, bucketInventory, bucketDevices, bucketMeta,
		} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("store: bucket %s: %w", name, err)
			}
		}
		return tx.Bucket(bucketMeta).Put([]byte("schema_version"), []byte("1"))
	})
}

// Close flushes and closes the database. It is idempotent.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Tx is a transaction handle exposing typed helpers. It is only valid for the
// duration of the Update or View call that produced it.
type Tx struct {
	bt *bolt.Tx
}

// Update runs fn inside a single writable transaction, committing atomically.
func (s *Store) Update(fn func(tx *Tx) error) error {
	return s.db.Update(func(bt *bolt.Tx) error { return fn(&Tx{bt: bt}) })
}

// View runs fn inside a read-only transaction.
func (s *Store) View(fn func(tx *Tx) error) error {
	return s.db.View(func(bt *bolt.Tx) error { return fn(&Tx{bt: bt}) })
}

// ErrNotFound is returned when a requested key has no value.
var ErrNotFound = errors.New("store: not found")

func (t *Tx) bucket(name []byte) (*bolt.Bucket, error) {
	b := t.bt.Bucket(name)
	if b == nil {
		return nil, fmt.Errorf("store: missing bucket %s", name)
	}
	return b, nil
}

// GetJSON reads the JSON value at key in bucket into out. It returns
// (false, nil) when the key is absent.
func (t *Tx) GetJSON(bucket, key []byte, out any) (bool, error) {
	b, err := t.bucket(bucket)
	if err != nil {
		return false, err
	}
	raw := b.Get(key)
	if raw == nil {
		return false, nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("store: decode %s/%s: %w", bucket, key, err)
	}
	return true, nil
}

// PutJSON writes v as JSON at key in bucket.
func (t *Tx) PutJSON(bucket, key []byte, v any) error {
	b, err := t.bucket(bucket)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("store: encode %s/%s: %w", bucket, key, err)
	}
	return b.Put(key, raw)
}

// Delete removes key from bucket.
func (t *Tx) Delete(bucket, key []byte) error {
	b, err := t.bucket(bucket)
	if err != nil {
		return err
	}
	return b.Delete(key)
}

// ForEach iterates all key/value pairs in bucket in ascending key order,
// invoking fn with the raw bytes for each entry.
func (t *Tx) ForEach(bucket []byte, fn func(k, v []byte) error) error {
	b, err := t.bucket(bucket)
	if err != nil {
		return err
	}
	return b.ForEach(fn)
}

// decode is a package-internal JSON decode helper used by typed readers.
func decode(raw []byte, out any) error {
	return json.Unmarshal(raw, out)
}
