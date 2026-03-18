package secretstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const serviceName = "certyn-cli"

var ErrSecretNotFound = errors.New("secret not found")
var keyringSet = keyring.Set
var keyringGet = keyring.Get
var keyringDelete = keyring.Delete

type Store interface {
	Get(ref string) (string, error)
	Set(ref, value string) error
	Delete(ref string) error
}

type HybridStore struct {
	file *fileStore
}

func NewHybridStore(configPath string) *HybridStore {
	secretPath := filepath.Join(filepath.Dir(configPath), "secrets.yaml")
	return &HybridStore{file: &fileStore{path: secretPath}}
}

func (s *HybridStore) Get(ref string) (string, error) {
	if ref == "" {
		return "", ErrSecretNotFound
	}

	if value, err := keyringGet(serviceName, ref); err == nil && value != "" {
		return value, nil
	}

	return s.file.Get(ref)
}

func (s *HybridStore) Set(ref, value string) error {
	if ref == "" {
		return errors.New("secret ref is required")
	}
	if value == "" {
		return errors.New("secret value is required")
	}

	// Try keyring first, but verify read-after-write.
	// Some environments can report successful writes yet fail reads.
	if err := keyringSet(serviceName, ref, value); err == nil {
		if readBack, getErr := keyringGet(serviceName, ref); getErr == nil && readBack == value {
			return nil
		}
	}

	return s.file.Set(ref, value)
}

func (s *HybridStore) Delete(ref string) error {
	if ref == "" {
		return errors.New("secret ref is required")
	}

	if err := keyringDelete(serviceName, ref); err == nil {
		return nil
	}

	return s.file.Delete(ref)
}

type fileSecrets struct {
	Secrets map[string]string `yaml:"secrets"`
}

type fileStore struct {
	path string
}

func (s *fileStore) Get(ref string) (string, error) {
	cfg, err := s.read()
	if err != nil {
		return "", err
	}
	value, ok := cfg.Secrets[ref]
	if !ok || value == "" {
		return "", ErrSecretNotFound
	}
	return value, nil
}

func (s *fileStore) Set(ref, value string) error {
	cfg, err := s.read()
	if err != nil {
		return err
	}
	if cfg.Secrets == nil {
		cfg.Secrets = make(map[string]string)
	}
	cfg.Secrets[ref] = value
	return s.write(cfg)
}

func (s *fileStore) Delete(ref string) error {
	cfg, err := s.read()
	if err != nil {
		return err
	}
	delete(cfg.Secrets, ref)
	return s.write(cfg)
}

func (s *fileStore) read() (*fileSecrets, error) {
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return &fileSecrets{Secrets: make(map[string]string)}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read secrets file: %w", err)
	}
	if len(data) == 0 {
		return &fileSecrets{Secrets: make(map[string]string)}, nil
	}
	var cfg fileSecrets
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse secrets file: %w", err)
	}
	if cfg.Secrets == nil {
		cfg.Secrets = make(map[string]string)
	}
	return &cfg, nil
}

func (s *fileStore) write(cfg *fileSecrets) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create secrets dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal secrets file: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write secrets file: %w", err)
	}
	return nil
}
