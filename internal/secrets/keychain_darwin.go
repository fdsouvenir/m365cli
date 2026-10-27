//go:build darwin

package secrets

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/config"
	"github.com/keybase/go-keychain"
)

const (
	// errSecInteractionNotAllowed is macOS Security framework error -25308.
	errSecInteractionNotAllowed = "-25308"
)

var (
	errKeychainPathUnknown = errors.New("cannot determine login keychain path")
	errKeychainNoTTY       = errors.New("keychain is locked and no TTY available for password prompt")
	errKeychainUnlock      = errors.New("unlock keychain: incorrect password or keychain error")

	execLookPathFunc       = exec.LookPath
	execCommandContextFunc = exec.CommandContext
)

// IsKeychainLockedError returns true if the error string indicates a locked keychain.
func IsKeychainLockedError(errStr string) bool {
	return strings.Contains(errStr, errSecInteractionNotAllowed)
}

func nativeKeychainAvailable() bool {
	_, err := execLookPathFunc("security")
	return err == nil
}

// Keep the existing service/account mapping so saved credentials remain readable.
func nativeKeychainQuery(key string) keychain.Item {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(config.AppName)
	item.SetAccount(nativeKeychainAccount(key))
	return item
}

func setNativeKeychainSecret(key string, value []byte) error {
	item := nativeKeychainQuery(key)
	item.SetData(value)
	err := keychain.AddItem(item)
	if errors.Is(err, keychain.ErrorDuplicateItem) {
		update := keychain.NewItem()
		update.SetData(value)
		err = keychain.UpdateItem(nativeKeychainQuery(key), update)
	}
	if err != nil {
		return fmt.Errorf("store keychain secret: %w", err)
	}
	return nil
}

func getNativeKeychainSecret(key string) ([]byte, error) {
	item := nativeKeychainQuery(key)
	item.SetMatchLimit(keychain.MatchLimitOne)
	item.SetReturnData(true)
	results, err := keychain.QueryItem(item)
	if errors.Is(err, keychain.ErrorItemNotFound) || (err == nil && len(results) == 0) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("read keychain secret: %w", err)
	}
	return results[0].Data, nil
}

func deleteNativeKeychainSecret(key string) error {
	err := keychain.DeleteItem(nativeKeychainQuery(key))
	if err != nil && !errors.Is(err, keychain.ErrorItemNotFound) {
		return fmt.Errorf("delete keychain secret: %w", err)
	}
	return nil
}

func nativeKeychainAccount(key string) string {
	return config.AppName + ":" + base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(key)))
}

// loginKeychainPath returns the path to the user's login keychain.
func loginKeychainPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}

// CheckKeychainLocked checks if the login keychain is locked.
func CheckKeychainLocked() bool {
	path := loginKeychainPath()
	if path == "" {
		return false
	}

	cmd := execCommandContextFunc(context.Background(), "security", "show-keychain-info", path) //nolint:gosec // path is from os.UserHomeDir
	return cmd.Run() != nil
}

// UnlockKeychain prompts for password and unlocks the login keychain.
func UnlockKeychain() error {
	path := loginKeychainPath()
	if path == "" {
		return errKeychainPathUnknown
	}

	if password, passwordSet := keyringPassword(); passwordSet {
		return unlockKeychainWithPassword(path, password)
	}

	if !isTTY(os.Stdin) {
		return fmt.Errorf("%w\n\nTo unlock manually, run:\n  security unlock-keychain ~/Library/Keychains/login.keychain-db", errKeychainNoTTY)
	}

	fmt.Fprint(os.Stderr, "Keychain is locked. Enter your macOS login password to unlock: ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	return unlockKeychainWithPassword(path, strings.TrimRight(line, "\r\n"))
}

func unlockKeychainWithPassword(path string, password string) error {
	cmd := execCommandContextFunc(context.Background(), "security", "unlock-keychain", path) //nolint:gosec // path is from os.UserHomeDir
	cmd.Stdin = strings.NewReader(password + "\n")

	if err := cmd.Run(); err != nil {
		return errKeychainUnlock
	}
	return nil
}

// EnsureKeychainAccess checks if the keychain is accessible and unlocks if needed.
func EnsureKeychainAccess() error {
	if !CheckKeychainLocked() {
		return nil
	}
	return UnlockKeychain()
}

func isTTY(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}
