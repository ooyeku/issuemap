package integration

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain ensures any accidental .issuemap created in the integration package directory is cleaned up
func TestMain(m *testing.M) {
	// Pre-clean in case of previous residue
	if wd, err := os.Getwd(); err == nil {
		_ = os.RemoveAll(filepath.Join(wd, ".issuemap"))
	}

	code := m.Run()

	// Post-clean to ensure no leftovers
	if wd, err := os.Getwd(); err == nil {
		_ = os.RemoveAll(filepath.Join(wd, ".issuemap"))
	}

	os.Exit(code)
}
