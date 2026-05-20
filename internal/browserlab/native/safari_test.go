package native

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSafariDetectorReportsReadyNativeRuntime(t *testing.T) {
	detector := SafariDetector{
		OS: "darwin",
		Runner: &fakeRunner{
			outputs: map[string]string{
				"/usr/bin/defaults read /Applications/Safari.app/Contents/Info CFBundleShortVersionString": "17.5\n",
				"/usr/bin/safaridriver --version": "Included with Safari 17.5\n",
			},
		},
	}

	runtime := detector.Detect(context.Background())

	if runtime.ID != "safari" {
		t.Fatalf("ID = %q, want safari", runtime.ID)
	}
	if runtime.Kind != "native" {
		t.Fatalf("Kind = %q, want native", runtime.Kind)
	}
	if runtime.Installable {
		t.Fatal("Installable = true, want false")
	}
	if runtime.Status != "ready" {
		t.Fatalf("Status = %q, want ready", runtime.Status)
	}
	if runtime.BrowserVersion != "17.5" {
		t.Fatalf("BrowserVersion = %q, want 17.5", runtime.BrowserVersion)
	}
	if !runtime.DriverAvailable {
		t.Fatal("DriverAvailable = false, want true")
	}
	if runtime.DriverVersion != "Included with Safari 17.5" {
		t.Fatalf("DriverVersion = %q, want safaridriver output", runtime.DriverVersion)
	}
	if runtime.OutOfScope != SafariOldVersionsOutOfScope {
		t.Fatalf("OutOfScope = %q, want Safari old-version guidance", runtime.OutOfScope)
	}
}

func TestSafariDetectorReportsSetupRequiredWhenSafariExistsButDriverIsMissing(t *testing.T) {
	detector := SafariDetector{
		OS: "darwin",
		Runner: &fakeRunner{
			outputs: map[string]string{
				"/usr/bin/defaults read /Applications/Safari.app/Contents/Info CFBundleShortVersionString": "16.6\n",
			},
			errors: map[string]error{
				"/usr/bin/safaridriver --version": errors.New("safaridriver not found"),
			},
		},
	}

	runtime := detector.Detect(context.Background())

	if runtime.Status != "setup_required" {
		t.Fatalf("Status = %q, want setup_required", runtime.Status)
	}
	if runtime.BrowserAvailable != true {
		t.Fatal("BrowserAvailable = false, want true")
	}
	if runtime.DriverAvailable {
		t.Fatal("DriverAvailable = true, want false")
	}
	if len(runtime.SetupGuidance) == 0 {
		t.Fatal("SetupGuidance is empty, want safaridriver guidance")
	}
}

func TestSafariDetectorReportsUnavailableOffMacOSWithoutCommands(t *testing.T) {
	runner := fakeRunner{}
	detector := SafariDetector{
		OS:     "linux",
		Runner: &runner,
	}

	runtime := detector.Detect(context.Background())

	if runtime.Status != "unavailable" {
		t.Fatalf("Status = %q, want unavailable", runtime.Status)
	}
	if runtime.BrowserAvailable {
		t.Fatal("BrowserAvailable = true, want false")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("commands = %v, want none", runner.calls)
	}
}

func TestSafariDetectorUsesExpectedCommands(t *testing.T) {
	runner := fakeRunner{
		outputs: map[string]string{
			"/usr/bin/defaults read /Applications/Safari.app/Contents/Info CFBundleShortVersionString": "17.4\n",
			"/usr/bin/safaridriver --version": "Included with Safari 17.4\n",
		},
	}
	detector := SafariDetector{OS: "darwin", Runner: &runner}

	_ = detector.Detect(context.Background())

	want := []string{
		"/usr/bin/defaults read /Applications/Safari.app/Contents/Info CFBundleShortVersionString",
		"/usr/bin/safaridriver --version",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %v, want %v", runner.calls, want)
	}
}

type fakeRunner struct {
	outputs map[string]string
	errors  map[string]error
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := commandKey(name, args...)
	f.calls = append(f.calls, key)
	if err := f.errors[key]; err != nil {
		return "", err
	}
	return f.outputs[key], nil
}

func commandKey(name string, args ...string) string {
	parts := append([]string{name}, args...)
	return strings.Join(parts, " ")
}
