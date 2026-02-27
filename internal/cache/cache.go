package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	db     *sql.DB
	ttl    time.Duration
}

type CacheEntry struct {
	Key        string
	Value      string
	ExpiresAt  time.Time
}

func NewDB(ttlHours int) (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(home, ".cache", "depsradar")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(cacheDir, "cache.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_expires ON cache(expires_at);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &DB{
		db:  db,
		ttl: time.Duration(ttlHours) * time.Hour,
	}, nil
}

func (c *DB) Get(key string) (string, bool) {
	var value string
	var expiresAt int64

	err := c.db.QueryRow("SELECT value, expires_at FROM cache WHERE key = ?", key).Scan(&value, &expiresAt)
	if err != nil {
		return "", false
	}

	if time.Now().Unix() > expiresAt {
		c.db.Exec("DELETE FROM cache WHERE key = ?", key)
		return "", false
	}

	return value, true
}

func (c *DB) Set(key, value string) {
	expiresAt := time.Now().Add(c.ttl).Unix()
	c.db.Exec("INSERT OR REPLACE INTO cache (key, value, expires_at) VALUES (?, ?, ?)", key, value, expiresAt)
}

func (c *DB) Delete(key string) {
	c.db.Exec("DELETE FROM cache WHERE key = ?", key)
}

func (c *DB) Clean() {
	c.db.Exec("DELETE FROM cache WHERE expires_at < ?", time.Now().Unix())
}

func (c *DB) Close() error {
	return c.db.Close()
}

func MakeKey(ecosystem, name, version string) string {
	return fmt.Sprintf("%s:%s:%s", ecosystem, name, version)
}
