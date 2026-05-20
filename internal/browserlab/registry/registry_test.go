package registry

import "testing"

func TestFileStorePersistsInstalledBrowserProvenance(t *testing.T) {
	store := NewFileStore(t.TempDir())
	record := BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	}

	installed, err := store.Save(record)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if installed.AlreadyInstalled {
		t.Fatal("first save reported already installed")
	}

	reloaded := NewFileStore(store.Root())
	records, err := reloaded.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	got := records[0]
	if got.Family != record.Family ||
		got.Version != record.Version ||
		got.ImageTag != record.ImageTag ||
		got.Platform != record.Platform ||
		got.Source != record.Source ||
		!got.Enabled {
		t.Fatalf("record = %+v, want exact persisted provenance %+v", got, record)
	}
}

func TestFileStoreDisablesInstalledBrowser(t *testing.T) {
	store := NewFileStore(t.TempDir())
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	_, err := store.Save(BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: imageTag,
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	disabled, err := store.Disable(imageTag)
	if err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("disabled record Enabled = true, want false")
	}
	if disabled.ImageTag != imageTag || disabled.Family != "chrome" || disabled.Version != "119.0" {
		t.Fatalf("disabled record = %+v, want same browser provenance", disabled)
	}

	reloaded := NewFileStore(store.Root())
	record, ok, err := reloaded.FindByImageTag(imageTag)
	if err != nil {
		t.Fatalf("FindByImageTag() error = %v", err)
	}
	if !ok {
		t.Fatal("disabled browser was removed from registry")
	}
	if record.Enabled {
		t.Fatalf("persisted record Enabled = true, want false")
	}
}
