package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
)

func TestLaunchAgentPlistContainsUserDaemonMetadata(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	metadata, err := NewLaunchAgentMetadata(appSupport, "/Applications/BrowserLab.app/Contents/MacOS/browserlabd")
	if err != nil {
		t.Fatalf("NewLaunchAgentMetadata() error = %v", err)
	}
	plist, err := metadata.Plist()
	if err != nil {
		t.Fatalf("Plist() error = %v", err)
	}
	text := string(plist)

	for _, want := range []string{
		"<key>Label</key>",
		"<string>com.browserlab.daemon</string>",
		"<key>ProgramArguments</key>",
		"<string>/Applications/BrowserLab.app/Contents/MacOS/browserlabd</string>",
		"<key>BROWSERLAB_HOME</key>",
		"<string>" + appSupport.Paths.Root + "</string>",
		"<key>StandardOutPath</key>",
		"<string>" + filepath.Join(appSupport.Paths.LogsDir, "browserlabd.out.log") + "</string>",
		"<key>StandardErrorPath</key>",
		"<string>" + filepath.Join(appSupport.Paths.LogsDir, "browserlabd.err.log") + "</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("plist missing %q:\n%s", want, text)
		}
	}
}

func TestManagerStartUsesUserLaunchctlDomain(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	runner := &recordingRunner{}
	manager := Manager{
		AppSupport: appSupport,
		DaemonPath: "/tmp/browserlabd",
		Runner:     runner,
		UID:        func() int { return 501 },
	}

	if _, err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	want := []string{"bootstrap", "gui/501", filepath.Join(home, "Library", "LaunchAgents", "com.browserlab.daemon.plist")}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("launchctl args = %#v, want %#v", runner.args, want)
	}
}

func TestManagerStopReportsLaunchctlFailure(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	manager := Manager{
		AppSupport: appSupport,
		DaemonPath: "/tmp/browserlabd",
		Runner: failingRunner{
			output: "Boot-out failed: No such process",
			err:    errors.New("exit status 36"),
		},
		UID: func() int { return 501 },
	}

	_, err = manager.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop() error = nil, want launchctl failure")
	}
	var commandErr CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Stop() error = %T, want CommandError", err)
	}
	if commandErr.Operation != "stop daemon" {
		t.Fatalf("operation = %q, want stop daemon", commandErr.Operation)
	}
	if !strings.Contains(commandErr.Error(), "Boot-out failed") {
		t.Fatalf("error = %q, want launchctl output", commandErr.Error())
	}
}

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return "", nil
}

type failingRunner struct {
	output string
	err    error
}

func (r failingRunner) Run(_ context.Context, _ string, _ ...string) (string, error) {
	return r.output, r.err
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
