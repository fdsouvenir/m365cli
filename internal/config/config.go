package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"time"
)

type ProfileRecord struct {
	Name                    string   `json:"name"`
	Audience                string   `json:"audience"`
	ClientID                string   `json:"client_id"`
	Authority               string   `json:"authority,omitempty"`
	TenantID                string   `json:"tenant_id,omitempty"`
	AccountID               string   `json:"account_id,omitempty"`
	Username                string   `json:"username,omitempty"`
	AuthMode                string   `json:"auth_mode,omitempty"`
	DelegatedScopeWorkloads []string `json:"delegated_scope_workloads,omitempty"`
	AppOnlyUser             string   `json:"app_only_user,omitempty"`
	Active                  bool     `json:"active,omitempty"`
}

type File struct {
	KeyringBackend  string                   `json:"keyring_backend,omitempty"`
	DefaultTimezone string                   `json:"default_timezone,omitempty"`
	AccountAliases  map[string]string        `json:"account_aliases,omitempty"`
	AccountClients  map[string]string        `json:"account_clients,omitempty"`
	ClientDomains   map[string]string        `json:"client_domains,omitempty"`
	Profiles        map[string]ProfileRecord `json:"profiles,omitempty"`
	ActiveProfile   string                   `json:"active_profile,omitempty"`
}

var (
	configLockRetryInterval = 10 * time.Millisecond
	configLockTimeout       = 5 * time.Second
)

func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func WriteConfig(cfg File) error {
	return withConfigLock(func() error { return writeConfig(cfg) })
}

func writeConfig(cfg File) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config json: %w", err)
	}

	b = append(b, '\n')
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit config: %w", err)
	}

	return nil
}

func ConfigExists() (bool, error) {
	path, err := ConfigPath()
	if err != nil {
		return false, err
	}

	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}

		return false, fmt.Errorf("stat config: %w", statErr)
	}

	return true, nil
}

func ReadConfig() (File, error) {
	var cfg File
	err := withConfigLock(func() error { var err error; cfg, err = readConfig(); return err })
	return cfg, err
}

func readConfig() (File, error) {
	path, err := ConfigPath()
	if err != nil {
		return File{}, err
	}
	cfg := File{Profiles: map[string]ProfileRecord{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileRecord{}
	}
	return cfg, nil
}

// UpdateConfig holds one lock across the entire read-modify-write transaction.
func UpdateConfig(update func(*File) error) error {
	return withConfigLock(func() error {
		cfg, err := readConfig()
		if err != nil {
			return err
		}
		if err := update(&cfg); err != nil {
			return err
		}
		return writeConfig(cfg)
	})
}

func withConfigLock(fn func() error) error {
	if fn == nil {
		return nil
	}

	dir, err := EnsureDir()
	if err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}
	lockPath := filepath.Join(dir, "config.lock")
	deadline := time.Now().Add(configLockTimeout)

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open config lock: %w", err)
	}
	defer lockFile.Close()
	for {
		err = unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
			return fn()
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return fmt.Errorf("acquire config lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("acquire config lock timeout: %s", lockPath)
		}
		time.Sleep(configLockRetryInterval)
	}
}
