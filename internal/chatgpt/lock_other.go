//go:build !unix

package chatgpt

// lockFile is a no-op where advisory file locks are unavailable; refreshes
// are still serialized within one process.
func lockFile(string) (func(), error) {
	return func() {}, nil
}
