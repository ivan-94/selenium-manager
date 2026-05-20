package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultSource = "selenium-dockerhub"
	registryFile  = "browser-registry.json"
)

type BrowserRecord struct {
	Family      string `json:"family"`
	Version     string `json:"version"`
	ImageTag    string `json:"imageTag"`
	Platform    string `json:"platform"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
	InstalledAt string `json:"installedAt"`
}

type SaveResult struct {
	Record           BrowserRecord `json:"record"`
	AlreadyInstalled bool          `json:"alreadyInstalled"`
}

type FileStore struct {
	root string
	path string
}

func NewFileStore(root string) FileStore {
	root = filepath.Clean(root)
	return FileStore{
		root: root,
		path: filepath.Join(root, "config", registryFile),
	}
}

func (store FileStore) Root() string {
	return store.root
}

func (store FileStore) List() ([]BrowserRecord, error) {
	file, err := store.read()
	if err != nil {
		return nil, err
	}
	sortRecords(file.Browsers)
	return file.Browsers, nil
}

func (store FileStore) FindByImageTag(imageTag string) (BrowserRecord, bool, error) {
	imageTag = strings.TrimSpace(imageTag)
	records, err := store.List()
	if err != nil {
		return BrowserRecord{}, false, err
	}
	for _, record := range records {
		if record.ImageTag == imageTag {
			return record, true, nil
		}
	}
	return BrowserRecord{}, false, nil
}

func (store FileStore) Save(record BrowserRecord) (SaveResult, error) {
	record = normalizeRecord(record)
	if err := validateRecord(record); err != nil {
		return SaveResult{}, err
	}

	file, err := store.read()
	if err != nil {
		return SaveResult{}, err
	}

	for index, existing := range file.Browsers {
		if existing.ImageTag == record.ImageTag {
			if record.InstalledAt == "" {
				record.InstalledAt = existing.InstalledAt
			}
			file.Browsers[index] = record
			if err := store.write(file); err != nil {
				return SaveResult{}, err
			}
			return SaveResult{Record: record, AlreadyInstalled: true}, nil
		}
	}

	if record.InstalledAt == "" {
		record.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	}
	file.Browsers = append(file.Browsers, record)
	sortRecords(file.Browsers)
	if err := store.write(file); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Record: record}, nil
}

type registryDocument struct {
	Browsers []BrowserRecord `json:"browsers"`
}

func (store FileStore) read() (registryDocument, error) {
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return registryDocument{}, nil
	}
	if err != nil {
		return registryDocument{}, fmt.Errorf("read browser registry: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return registryDocument{}, nil
	}

	var file registryDocument
	if err := json.Unmarshal(data, &file); err != nil {
		return registryDocument{}, fmt.Errorf("decode browser registry: %w", err)
	}
	return file, nil
}

func (store FileStore) write(file registryDocument) error {
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("create browser registry directory: %w", err)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode browser registry: %w", err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(filepath.Dir(store.path), ".browser-registry-*.json")
	if err != nil {
		return fmt.Errorf("create browser registry temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write browser registry temp file: %w", err)
	}
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure browser registry temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close browser registry temp file: %w", err)
	}
	if err := os.Rename(tempPath, store.path); err != nil {
		return fmt.Errorf("replace browser registry: %w", err)
	}
	return nil
}

func normalizeRecord(record BrowserRecord) BrowserRecord {
	record.Family = strings.ToLower(strings.TrimSpace(record.Family))
	record.Version = strings.TrimSpace(record.Version)
	record.ImageTag = strings.TrimSpace(record.ImageTag)
	record.Platform = strings.TrimSpace(record.Platform)
	record.Source = strings.TrimSpace(record.Source)
	if record.Source == "" {
		record.Source = defaultSource
	}
	return record
}

func validateRecord(record BrowserRecord) error {
	if record.Family == "" {
		return fmt.Errorf("browser family is required")
	}
	if record.Version == "" {
		return fmt.Errorf("browser version is required")
	}
	if record.ImageTag == "" {
		return fmt.Errorf("image tag is required")
	}
	if record.Platform == "" {
		return fmt.Errorf("platform is required")
	}
	if record.Source == "" {
		return fmt.Errorf("source is required")
	}
	return nil
}

func sortRecords(records []BrowserRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Family != records[j].Family {
			return records[i].Family < records[j].Family
		}
		if records[i].Version != records[j].Version {
			return records[i].Version > records[j].Version
		}
		return records[i].ImageTag < records[j].ImageTag
	})
}
