package gitlab

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type mrLockKey struct{ identity string }

// LockMR serializes short read/modify/write sections across the daemon and its
// child processes. Reentrant context admission avoids locking a nested write
// twice. Locks are released by the OS on process exit, including crashes.
func (c *Client) LockMR(ctx context.Context, project string, iid int) (context.Context, func(), error) {
	key := mrLockKey{fmt.Sprintf("%s/%s!%d", c.baseURL, project, iid)}
	if held, _ := ctx.Value(key).(bool); held {
		return ctx, func() {}, nil
	}
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("nickpit-mr-locks-%d", os.Getuid()))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ctx, nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return ctx, nil, err
	}
	if !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return ctx, nil, fmt.Errorf("unsafe MR lock directory %s", dir)
	}
	name := filepath.Join(dir, fmt.Sprintf("%x.lock", sha256.Sum256([]byte(key.identity))))
	if info, err := os.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return ctx, nil, fmt.Errorf("unsafe MR lock file")
	}
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return ctx, nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()
			return ctx, nil, err
		}
		locked, err := tryMRFileLock(f)
		if err != nil {
			_ = f.Close()
			return ctx, nil, err
		}
		if locked {
			return context.WithValue(ctx, key, true), func() { _ = f.Close() }, nil
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return ctx, nil, ctx.Err()
		case <-timer.C:
		}
	}
}
