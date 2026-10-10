//go:build !unix && !windows

package chatgpt

// lockFile is a no-op on platforms with neither flock nor LockFileEx (wasm,
// plan9); refreshes are still serialized within one process.
func lockFile(string) (func(), error) {
	return func() {}, nil
}
