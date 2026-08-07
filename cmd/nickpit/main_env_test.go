package main

import (
	"os"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

// The CLI resolves its profile and credentials from the ambient environment as
// well as from flags, so the tests start from an environment that carries
// neither.
func TestMain(m *testing.M) {
	testutil.ClearAmbientEnv()
	os.Exit(m.Run())
}
