package secretstore

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestHybridStore_SetFallsBackToFileWhenKeyringSetFails(t *testing.T) {
	origSet := keyringSet
	origGet := keyringGet
	t.Cleanup(func() {
		keyringSet = origSet
		keyringGet = origGet
	})

	keyringSet = func(service, user, pass string) error {
		return errors.New("keyring unavailable")
	}
	keyringGet = func(service, user string) (string, error) {
		return "", errors.New("keyring unavailable")
	}

	store := NewHybridStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err := store.Set("dev_key", "secret-value"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	value, err := store.Get("dev_key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("unexpected secret value: got %q", value)
	}
}

func TestHybridStore_SetFallsBackToFileWhenReadAfterWriteFails(t *testing.T) {
	origSet := keyringSet
	origGet := keyringGet
	t.Cleanup(func() {
		keyringSet = origSet
		keyringGet = origGet
	})

	keyringSet = func(service, user, pass string) error {
		return nil
	}
	keyringGet = func(service, user string) (string, error) {
		return "", errors.New("keyring read failed")
	}

	store := NewHybridStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err := store.Set("dev_key", "secret-value"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	value, err := store.Get("dev_key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("unexpected secret value: got %q", value)
	}
}

func TestHybridStore_UsesKeyringWhenReadable(t *testing.T) {
	origSet := keyringSet
	origGet := keyringGet
	t.Cleanup(func() {
		keyringSet = origSet
		keyringGet = origGet
	})

	keyringSet = func(service, user, pass string) error {
		return nil
	}
	keyringGet = func(service, user string) (string, error) {
		return "secret-value", nil
	}

	tempDir := t.TempDir()
	store := NewHybridStore(filepath.Join(tempDir, "config.yaml"))
	if err := store.Set("dev_key", "secret-value"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	value, err := store.Get("dev_key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("unexpected secret value: got %q", value)
	}

	_, fileErr := store.file.Get("dev_key")
	if !errors.Is(fileErr, ErrSecretNotFound) {
		t.Fatalf("expected no file fallback write when keyring is readable, got %v", fileErr)
	}
}

func TestHybridStore_DeleteFallsBackToFile(t *testing.T) {
	origSet := keyringSet
	origGet := keyringGet
	origDelete := keyringDelete
	t.Cleanup(func() {
		keyringSet = origSet
		keyringGet = origGet
		keyringDelete = origDelete
	})

	keyringSet = func(service, user, pass string) error {
		return errors.New("keyring unavailable")
	}
	keyringGet = func(service, user string) (string, error) {
		return "", errors.New("keyring unavailable")
	}
	keyringDelete = func(service, user string) error {
		return errors.New("keyring unavailable")
	}

	store := NewHybridStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err := store.Set("dev_key", "secret-value"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := store.Delete("dev_key"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Get("dev_key"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected secret to be deleted, got %v", err)
	}
}
