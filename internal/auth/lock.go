package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/fdsouvenir/m365cli/internal/config"
	"github.com/fdsouvenir/m365cli/internal/profile"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"time"
)

// Serialize token refresh/rotation for one profile, without holding the config lock
// across network requests. The kernel releases ownership even after SIGKILL.
func lockProfile(ctx context.Context, name string) (func(), error) {
	name, err := profile.NormalizeName(name)
	if err != nil {
		return nil, err
	}
	dir, err := config.EnsureDir()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "token-"+name+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = f.Close()
			return nil, fmt.Errorf("lock token cache: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
