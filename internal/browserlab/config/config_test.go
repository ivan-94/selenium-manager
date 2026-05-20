package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAppSupportInitializesTokenConfigAndLogConventions(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())

	state, err := EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	for name, dir := range map[string]string{
		"root":      state.Paths.Root,
		"config":    state.Paths.ConfigDir,
		"logs":      state.Paths.LogsDir,
		"artifacts": state.Paths.ArtifactsDir,
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("%s dir was not created: %v", name, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s path is not a directory: %s", name, dir)
		}
	}

	if filepath.Base(state.Paths.TokenFile) != "token" {
		t.Fatalf("token file name = %q, want token", filepath.Base(state.Paths.TokenFile))
	}
	if state.Token == "" {
		t.Fatal("token was empty")
	}

	tokenInfo, err := os.Stat(state.Paths.TokenFile)
	if err != nil {
		t.Fatalf("token file was not created: %v", err)
	}
	if got := tokenInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("token permissions = %v, want 0600", got)
	}

	configInfo, err := os.Stat(state.Paths.ConfigFile)
	if err != nil {
		t.Fatalf("config file was not created: %v", err)
	}
	if got := configInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %v, want 0600", got)
	}

	second, err := EnsureAppSupport()
	if err != nil {
		t.Fatalf("second EnsureAppSupport() error = %v", err)
	}
	if second.Token != state.Token {
		t.Fatalf("token was not stable across initialization")
	}
}
