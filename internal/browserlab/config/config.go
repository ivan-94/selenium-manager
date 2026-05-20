package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appSupportEnv = "BROWSERLAB_HOME"
	appName       = "BrowserLab"
)

type AppSupport struct {
	Paths Paths
	Token string
}

type Paths struct {
	Root         string `json:"root"`
	ConfigDir    string `json:"configDir"`
	LogsDir      string `json:"logsDir"`
	ArtifactsDir string `json:"artifactsDir"`
	TokenFile    string `json:"tokenFile"`
	ConfigFile   string `json:"configFile"`
}

type FileConfig struct {
	API APIConfig `json:"api"`
}

type APIConfig struct {
	Listen string `json:"listen"`
}

func EnsureAppSupport() (AppSupport, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return AppSupport{}, err
	}

	for _, dir := range []string{paths.Root, paths.ConfigDir, paths.LogsDir, paths.ArtifactsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return AppSupport{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	if err := ensureConfigFile(paths.ConfigFile); err != nil {
		return AppSupport{}, err
	}

	token, err := ensureToken(paths.TokenFile)
	if err != nil {
		return AppSupport{}, err
	}

	return AppSupport{Paths: paths, Token: token}, nil
}

func ResolvePaths() (Paths, error) {
	root := os.Getenv(appSupportEnv)
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve home directory: %w", err)
		}
		root = filepath.Join(home, "Library", "Application Support", appName)
	}
	root = filepath.Clean(root)

	configDir := filepath.Join(root, "config")
	return Paths{
		Root:         root,
		ConfigDir:    configDir,
		LogsDir:      filepath.Join(root, "logs"),
		ArtifactsDir: filepath.Join(root, "artifacts"),
		TokenFile:    filepath.Join(configDir, "token"),
		ConfigFile:   filepath.Join(configDir, "config.json"),
	}, nil
}

func ensureConfigFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config file: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(FileConfig{API: APIConfig{Listen: "127.0.0.1:49321"}}); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func ensureToken(path string) (string, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(existing))
		if token == "" {
			return "", fmt.Errorf("token file %s is empty", path)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return "", fmt.Errorf("secure token file: %w", err)
		}
		return token, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read token file: %w", err)
	}

	token, err := newToken()
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ensureToken(path)
		}
		return "", fmt.Errorf("create token file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(token + "\n"); err != nil {
		return "", fmt.Errorf("write token file: %w", err)
	}
	return token, nil
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
