package auth

import (
	"path/filepath"
	"testing"
)

func TestLoadOrCreate_GeneratesAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets", "apikey.key")

	key1, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error: %v", err)
	}
	if len(key1) == 0 {
		t.Fatal("LoadOrCreate() returned empty key")
	}

	// A second call must reuse the persisted key, not regenerate it.
	key2, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() second call error: %v", err)
	}
	if key1 != key2 {
		t.Fatalf("LoadOrCreate() regenerated the key: %q != %q", key1, key2)
	}
}

func TestValid(t *testing.T) {
	if !Valid("secret", "secret") {
		t.Error("Valid(secret, secret) = false, want true")
	}
	if Valid("secret", "wrong") {
		t.Error("Valid(secret, wrong) = true, want false")
	}
	if Valid("", "") {
		t.Error("Valid(\"\", \"\") = true, want false (empty key must never validate)")
	}
	if Valid("secret", "") {
		t.Error("Valid(secret, \"\") = true, want false")
	}
}
