package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecretReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUNESCOUT_API_KEY_FILE", path)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "from-file" {
		t.Fatalf("APIKey = %q, want from-file", cfg.APIKey)
	}
}

func TestSecretEnvironmentTakesPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("from-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUNESCOUT_API_KEY", "from-env")
	t.Setenv("TUNESCOUT_API_KEY_FILE", path)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "from-env" {
		t.Fatalf("APIKey = %q, want from-env", cfg.APIKey)
	}
}

func TestSecretMissingFileFailsClosed(t *testing.T) {
	t.Setenv("TUNESCOUT_API_KEY_FILE", filepath.Join(t.TempDir(), "missing"))
	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv() succeeded with a missing secret file")
	}
}
