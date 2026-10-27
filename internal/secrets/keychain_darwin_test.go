//go:build darwin

package secrets

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Exercise the real storage boundary with synthetic data; never use saved tokens.
func TestNativeKeychainNeverPassesSecretInArgv(t *testing.T) {
	key := fmt.Sprintf("m365cli-keychain-regression-%d-%d", os.Getpid(), time.Now().UnixNano())
	value := []byte("synthetic-token-for-argv-regression")
	original := execCommandContextFunc
	execCommandContextFunc = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, arg := range args {
			if strings.Contains(arg, string(value)) {
				t.Error("secret exposed in child process arguments")
				return exec.CommandContext(ctx, "false")
			}
		}
		return original(ctx, name, args...)
	}
	defer func() { execCommandContextFunc = original }()
	defer deleteNativeKeychainSecret(key)
	if err := setNativeKeychainSecret(key, value); err != nil {
		t.Fatal(err)
	}
	got, err := getNativeKeychainSecret(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, value) {
		t.Fatal("stored value did not round-trip")
	}
	replacement := []byte("synthetic-replacement")
	if err := setNativeKeychainSecret(key, replacement); err != nil {
		t.Fatal(err)
	}
	got, err = getNativeKeychainSecret(key)
	if err != nil || !bytes.Equal(got, replacement) {
		t.Fatal("replacement did not round-trip", err)
	}
	if err := deleteNativeKeychainSecret(key); err != nil {
		t.Fatal(err)
	}
	if _, err := getNativeKeychainSecret(key); !os.IsNotExist(err) {
		t.Fatalf("deleted key still present: %v", err)
	}
}
