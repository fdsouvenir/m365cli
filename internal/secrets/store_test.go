package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/config"
)

func TestSetGetDeleteSecretFileBackend(t *testing.T) {
	home := t.TempDir()
	selected := filepath.Join(t.TempDir(), "selected-account")
	t.Setenv("M365CLI_CONFIG_DIR", selected)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("M365CLI_KEYRING_BACKEND", "file")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "test-only-password")

	if err := config.WriteConfig(config.File{}); err != nil {
		t.Fatalf("write selected config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(selected, "config.json")); err != nil {
		t.Fatalf("configuration did not use selected directory: %v", err)
	}

	key := "m365cli:test:key"
	value := []byte("hello-world")

	if err := SetSecret(key, value); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	got, err := GetSecret(key)
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if string(got) != string(value) {
		t.Fatalf("unexpected secret value: %q", string(got))
	}

	if err := DeleteSecret(key); err != nil {
		t.Fatalf("DeleteSecret failed: %v", err)
	}

	if _, err := GetSecret(key); err == nil {
		t.Fatal("expected error after deleting secret")
	}

	if _, err := os.Stat(filepath.Join(selected, "keyring")); err != nil {
		t.Fatalf("encrypted keyring did not use selected directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "m365cli")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("default configuration was touched: %v", err)
	}
	if os.Getenv("HOME") != home || os.Getenv("XDG_CONFIG_HOME") != filepath.Join(home, ".config") {
		t.Fatal("process home or global XDG context was changed")
	}
	t.Setenv("M365CLI_CONFIG_DIR", "relative/config")
	if _, err := config.Dir(); err == nil {
		t.Fatal("relative config directory accepted")
	}

}

func TestSetGetDeleteSecretKeychainBackendDispatch(t *testing.T) {
	t.Setenv("M365CLI_KEYRING_BACKEND", "keychain")
	restore := stubNativeKeychainFuncs(t)

	nativeKeychainAvailableFunc = func() bool { return true }

	ensureCalls := 0
	ensureKeychainAccessFunc = func() error {
		ensureCalls++
		return nil
	}

	var setKey string
	var setValue []byte
	setNativeKeychainSecretFunc = func(key string, value []byte) error {
		setKey = key
		setValue = append([]byte(nil), value...)
		return nil
	}

	getNativeKeychainSecretFunc = func(key string) ([]byte, error) {
		if key != "m365cli:test:key" {
			t.Fatalf("unexpected get key %q", key)
		}
		return []byte("value-from-keychain"), nil
	}

	deleteKey := ""
	deleteNativeKeychainSecretFunc = func(key string) error {
		deleteKey = key
		return nil
	}

	t.Cleanup(restore)

	if err := SetSecret("m365cli:test:key", []byte("hello-world")); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}
	if setKey != "m365cli:test:key" {
		t.Fatalf("unexpected set key: %q", setKey)
	}
	if string(setValue) != "hello-world" {
		t.Fatalf("unexpected set value: %q", string(setValue))
	}

	got, err := GetSecret("m365cli:test:key")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if string(got) != "value-from-keychain" {
		t.Fatalf("unexpected get value: %q", string(got))
	}

	if err := DeleteSecret("m365cli:test:key"); err != nil {
		t.Fatalf("DeleteSecret failed: %v", err)
	}
	if deleteKey != "m365cli:test:key" {
		t.Fatalf("unexpected delete key: %q", deleteKey)
	}

	if ensureCalls != 3 {
		t.Fatalf("expected EnsureKeychainAccess to be called 3 times, got %d", ensureCalls)
	}
}

func TestAutoBackendFallsBackToFileWhenNativeUnavailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("M365CLI_KEYRING_BACKEND", "auto")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "test-only-password")

	restore := stubNativeKeychainFuncs(t)
	nativeKeychainAvailableFunc = func() bool { return false }
	setNativeKeychainSecretFunc = func(_ string, _ []byte) error {
		t.Fatal("unexpected native keychain set call")
		return nil
	}
	getNativeKeychainSecretFunc = func(_ string) ([]byte, error) {
		t.Fatal("unexpected native keychain get call")
		return nil, nil
	}
	deleteNativeKeychainSecretFunc = func(_ string) error {
		t.Fatal("unexpected native keychain delete call")
		return nil
	}
	t.Cleanup(restore)

	key := "m365cli:test:auto-file"
	value := []byte("auto-file-value")
	if err := SetSecret(key, value); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	got, err := GetSecret(key)
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if string(got) != string(value) {
		t.Fatalf("unexpected secret value: %q", string(got))
	}

	if err := DeleteSecret(key); err != nil {
		t.Fatalf("DeleteSecret failed: %v", err)
	}
}

func TestResolveKeyringBackendInfoPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("M365CLI_KEYRING_BACKEND", "")

	info, err := ResolveKeyringBackendInfo()
	if err != nil {
		t.Fatalf("ResolveKeyringBackendInfo default failed: %v", err)
	}
	if info.Value != keyringBackendAuto || info.Source != keyringBackendSourceDefault {
		t.Fatalf("unexpected default backend info: %#v", info)
	}

	if err := config.WriteConfig(config.File{KeyringBackend: "file"}); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	info, err = ResolveKeyringBackendInfo()
	if err != nil {
		t.Fatalf("ResolveKeyringBackendInfo config failed: %v", err)
	}
	if info.Value != keyringBackendFile || info.Source != keyringBackendSourceConfig {
		t.Fatalf("unexpected config backend info: %#v", info)
	}

	t.Setenv("M365CLI_KEYRING_BACKEND", "keychain")
	info, err = ResolveKeyringBackendInfo()
	if err != nil {
		t.Fatalf("ResolveKeyringBackendInfo env failed: %v", err)
	}
	if info.Value != keyringBackendKeychain || info.Source != keyringBackendSourceEnv {
		t.Fatalf("unexpected env backend info: %#v", info)
	}
}

func TestSetSecretRejectsInvalidBackend(t *testing.T) {
	t.Setenv("M365CLI_KEYRING_BACKEND", "bogus")
	err := SetSecret("m365cli:test:key", []byte("value"))
	if err == nil {
		t.Fatal("expected SetSecret to fail for invalid backend")
	}
	if !errors.Is(err, errInvalidKeyringBackend) {
		t.Fatalf("expected errInvalidKeyringBackend, got %v", err)
	}
}

func TestSetSecretRejectsUnavailableNativeKeychain(t *testing.T) {
	t.Setenv("M365CLI_KEYRING_BACKEND", "keychain")
	restore := stubNativeKeychainFuncs(t)
	nativeKeychainAvailableFunc = func() bool { return false }
	t.Cleanup(restore)

	err := SetSecret("m365cli:test:key", []byte("value"))
	if err == nil {
		t.Fatal("expected SetSecret to fail for unavailable native keychain")
	}
	if !errors.Is(err, errNativeKeychainUnavailable) {
		t.Fatalf("expected errNativeKeychainUnavailable, got %v", err)
	}
}

func TestKeyringPasswordTreatsEmptyAsIntentional(t *testing.T) {
	t.Setenv("M365CLI_KEYRING_PASSWORD", "")
	password, ok := keyringPassword()
	if !ok {
		t.Fatal("expected empty keyring password to be treated as set")
	}
	if password != "" {
		t.Fatalf("expected empty password, got %q", password)
	}
}

func stubNativeKeychainFuncs(t *testing.T) func() {
	t.Helper()

	originalAvailable := nativeKeychainAvailableFunc
	originalEnsure := ensureKeychainAccessFunc
	originalSet := setNativeKeychainSecretFunc
	originalGet := getNativeKeychainSecretFunc
	originalDelete := deleteNativeKeychainSecretFunc

	return func() {
		nativeKeychainAvailableFunc = originalAvailable
		ensureKeychainAccessFunc = originalEnsure
		setNativeKeychainSecretFunc = originalSet
		getNativeKeychainSecretFunc = originalGet
		deleteNativeKeychainSecretFunc = originalDelete
	}
}

// Persisted headless credentials must not expose token plaintext; a wrong key
// must not be accepted. Existing round-trip tests did not inspect disk bytes.
func TestFileSecretEncryptedAtRest(t *testing.T) {
	secretHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", secretHome)
	t.Setenv("M365CLI_KEYRING_BACKEND", "file")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "test-only-password")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "test-only-password")
	value := []byte("distinct-refresh-token-for-regression")
	if err := SetSecret("regression", value); err != nil {
		t.Fatal(err)
	}
	path, err := secretPath("regression")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), string(value)) {
		t.Fatal("credential stored in plaintext")
	}
	t.Setenv("M365CLI_KEYRING_PASSWORD", "wrong-password")
	if _, err := GetSecret("regression"); err == nil {
		t.Fatal("wrong key accepted")
	}
}
