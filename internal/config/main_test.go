package config

import (
	"os"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

// Load consults the ambient environment for the active profile, API keys and
// forge tokens, so the tests below only describe their own fixtures once the
// developer's exported NICKPIT_*/API-key variables are out of the way.
func TestMain(m *testing.M) {
	testutil.ClearAmbientEnv()
	os.Exit(m.Run())
}
