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
