package service_test

import (
	"path/filepath"
	"testing"

	"arham-gateway/internal/service"
)

func TestRunSetupInvalidExplicitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentConfig := filepath.Join(tmpDir, "does-not-exist.toml")

	err := service.RunSetup(nonExistentConfig)
	if err == nil {
		t.Fatalf("expected RunSetup to fail when given non-existent explicit config, got nil")
	}
}
