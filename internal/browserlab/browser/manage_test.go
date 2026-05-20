package browser

import (
	"context"
	"errors"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

func TestManagerUninstallRemovesRegistryEntryWithoutDeletingImage(t *testing.T) {
	store := registry.NewFileStore(t.TempDir())
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	saveInstalledBrowser(t, store, imageTag)
	remover := &recordingImageRemover{}
	manager := Manager{Store: store, ImageRemover: remover}

	result, err := manager.Uninstall(context.Background(), UninstallRequest{ImageTag: imageTag})
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if result.Record.ImageTag != imageTag {
		t.Fatalf("uninstalled record = %+v, want %s", result.Record, imageTag)
	}
	if result.ImageDeleted {
		t.Fatal("ImageDeleted = true, want registry-only uninstall")
	}
	if len(remover.removed) != 0 {
		t.Fatalf("removed images = %#v, want none", remover.removed)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("registry records = %+v, want removed browser", records)
	}
}

func TestManagerUninstallRequiresConfirmationBeforeDeletingDockerImage(t *testing.T) {
	store := registry.NewFileStore(t.TempDir())
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	saveInstalledBrowser(t, store, imageTag)
	remover := &recordingImageRemover{}
	manager := Manager{Store: store, ImageRemover: remover}

	_, err := manager.Uninstall(context.Background(), UninstallRequest{
		ImageTag:    imageTag,
		DeleteImage: true,
	})
	var problem Problem
	if !errors.As(err, &problem) {
		t.Fatalf("Uninstall() error = %v, want Problem", err)
	}
	if problem.Code != "image_delete_confirmation_required" {
		t.Fatalf("problem code = %q, want image_delete_confirmation_required", problem.Code)
	}
	if len(remover.removed) != 0 {
		t.Fatalf("removed images = %#v, want none without confirmation", remover.removed)
	}
	if _, ok, err := store.FindByImageTag(imageTag); err != nil || !ok {
		t.Fatalf("browser should remain installed after unconfirmed delete, ok=%v err=%v", ok, err)
	}

	result, err := manager.Uninstall(context.Background(), UninstallRequest{
		ImageTag:           imageTag,
		DeleteImage:        true,
		ConfirmDeleteImage: true,
	})
	if err != nil {
		t.Fatalf("confirmed Uninstall() error = %v", err)
	}
	if !result.ImageDeleted {
		t.Fatal("ImageDeleted = false, want true after explicit confirmation")
	}
	if len(remover.removed) != 1 || remover.removed[0] != imageTag {
		t.Fatalf("removed images = %#v, want confirmed image deletion", remover.removed)
	}
}

func TestManagerUninstallFailsWhenActiveSessionsUseBrowser(t *testing.T) {
	store := registry.NewFileStore(t.TempDir())
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	saveInstalledBrowser(t, store, imageTag)
	remover := &recordingImageRemover{}
	manager := Manager{
		Store:        store,
		ImageRemover: remover,
		SessionChecker: staticSessionChecker{sessions: []ActiveSession{{
			ID:             "session-123",
			BrowserName:    "chrome",
			BrowserVersion: "119.0",
			ImageTag:       imageTag,
		}}},
	}

	_, err := manager.Uninstall(context.Background(), UninstallRequest{ImageTag: imageTag})
	var problem Problem
	if !errors.As(err, &problem) {
		t.Fatalf("Uninstall() error = %v, want Problem", err)
	}
	if problem.Code != "active_sessions_block_uninstall" {
		t.Fatalf("problem code = %q, want active_sessions_block_uninstall", problem.Code)
	}
	if len(remover.removed) != 0 {
		t.Fatalf("removed images = %#v, want none while active", remover.removed)
	}
	if _, ok, err := store.FindByImageTag(imageTag); err != nil || !ok {
		t.Fatalf("browser should remain installed while active, ok=%v err=%v", ok, err)
	}
}

func saveInstalledBrowser(t *testing.T, store registry.FileStore, imageTag string) {
	t.Helper()
	_, err := store.Save(registry.BrowserRecord{
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
}

type recordingImageRemover struct {
	removed []string
}

func (remover *recordingImageRemover) RemoveImage(_ context.Context, imageTag string) error {
	remover.removed = append(remover.removed, imageTag)
	return nil
}

type staticSessionChecker struct {
	sessions []ActiveSession
}

func (checker staticSessionChecker) ActiveSessionsForBrowser(context.Context, registry.BrowserRecord) ([]ActiveSession, error) {
	return checker.sessions, nil
}
