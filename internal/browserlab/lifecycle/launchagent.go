package lifecycle

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
)

const (
	Label     = "com.browserlab.daemon"
	PlistName = Label + ".plist"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type CommandError struct {
	Operation string
	Command   string
	Args      []string
	Output    string
	Err       error
}

func (e CommandError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", e.Operation, e.Command)
	for _, arg := range e.Args {
		fmt.Fprintf(&b, " %s", arg)
	}
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	if strings.TrimSpace(e.Output) != "" {
		fmt.Fprintf(&b, ": %s", strings.TrimSpace(e.Output))
	}
	return b.String()
}

func (e CommandError) Unwrap() error {
	return e.Err
}

type LaunchAgentMetadata struct {
	Label                string
	ProgramArguments     []string
	EnvironmentVariables map[string]string
	StandardOutPath      string
	StandardErrorPath    string
	RunAtLoad            bool
	KeepAlive            bool
}

func NewLaunchAgentMetadata(appSupport config.AppSupport, daemonPath string) (LaunchAgentMetadata, error) {
	if daemonPath == "" {
		return LaunchAgentMetadata{}, errors.New("daemon path is required")
	}
	if !filepath.IsAbs(daemonPath) {
		abs, err := filepath.Abs(daemonPath)
		if err != nil {
			return LaunchAgentMetadata{}, fmt.Errorf("resolve daemon path: %w", err)
		}
		daemonPath = abs
	}
	return LaunchAgentMetadata{
		Label:            Label,
		ProgramArguments: []string{daemonPath},
		EnvironmentVariables: map[string]string{
			"BROWSERLAB_HOME": appSupport.Paths.Root,
		},
		StandardOutPath:   filepath.Join(appSupport.Paths.LogsDir, "browserlabd.out.log"),
		StandardErrorPath: filepath.Join(appSupport.Paths.LogsDir, "browserlabd.err.log"),
		RunAtLoad:         true,
		KeepAlive:         true,
	}, nil
}

func (m LaunchAgentMetadata) Plist() ([]byte, error) {
	if m.Label == "" {
		return nil, errors.New("launch agent label is required")
	}
	if len(m.ProgramArguments) == 0 || m.ProgramArguments[0] == "" {
		return nil, errors.New("launch agent program arguments are required")
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	writeStringKey(&b, "Label", m.Label)
	writeStringArrayKey(&b, "ProgramArguments", m.ProgramArguments)
	writeBoolKey(&b, "RunAtLoad", m.RunAtLoad)
	writeBoolKey(&b, "KeepAlive", m.KeepAlive)
	writeStringKey(&b, "StandardOutPath", m.StandardOutPath)
	writeStringKey(&b, "StandardErrorPath", m.StandardErrorPath)
	writeStringDictKey(&b, "EnvironmentVariables", m.EnvironmentVariables)
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.Bytes(), nil
}

type Manager struct {
	AppSupport config.AppSupport
	DaemonPath string
	Runner     Runner
	UID        func() int
}

type Result struct {
	Message   string
	PlistPath string
	Output    string
}

func (m Manager) Install() (Result, error) {
	metadata, plistPath, err := m.metadata()
	if err != nil {
		return Result{}, err
	}
	plist, err := metadata.Plist()
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(plistPath, plist, 0o644); err != nil {
		return Result{}, fmt.Errorf("write LaunchAgent plist: %w", err)
	}
	return Result{
		Message:   "daemon LaunchAgent installed",
		PlistPath: plistPath,
	}, nil
}

func (m Manager) Start(ctx context.Context) (Result, error) {
	_, plistPath, err := m.metadata()
	if err != nil {
		return Result{}, err
	}
	args := []string{"bootstrap", m.userDomain(), plistPath}
	output, err := m.runner().Run(ctx, "launchctl", args...)
	if err != nil {
		return Result{}, CommandError{Operation: "start daemon", Command: "launchctl", Args: args, Output: output, Err: err}
	}
	return Result{Message: "daemon started", PlistPath: plistPath, Output: output}, nil
}

func (m Manager) Stop(ctx context.Context) (Result, error) {
	args := []string{"bootout", m.serviceTarget()}
	output, err := m.runner().Run(ctx, "launchctl", args...)
	if err != nil {
		return Result{}, CommandError{Operation: "stop daemon", Command: "launchctl", Args: args, Output: output, Err: err}
	}
	return Result{Message: "daemon stopped", Output: output}, nil
}

func (m Manager) Restart(ctx context.Context) (Result, error) {
	if _, err := m.Stop(ctx); err != nil {
		return Result{}, err
	}
	return m.Start(ctx)
}

func (m Manager) Status(ctx context.Context) (Result, error) {
	args := []string{"print", m.serviceTarget()}
	output, err := m.runner().Run(ctx, "launchctl", args...)
	if err != nil {
		return Result{}, CommandError{Operation: "inspect daemon", Command: "launchctl", Args: args, Output: output, Err: err}
	}
	return Result{Message: "daemon status", Output: output}, nil
}

func (m Manager) Logs() (Result, error) {
	var b strings.Builder
	appendLog := func(label string, path string) {
		fmt.Fprintf(&b, "==> %s (%s) <==\n", label, path)
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				b.WriteString("(log file does not exist yet)\n")
			} else {
				fmt.Fprintf(&b, "(could not read log file: %v)\n", err)
			}
			return
		}
		b.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	appendLog("stdout", filepath.Join(m.AppSupport.Paths.LogsDir, "browserlabd.out.log"))
	appendLog("stderr", filepath.Join(m.AppSupport.Paths.LogsDir, "browserlabd.err.log"))
	return Result{Message: "daemon logs", Output: b.String()}, nil
}

func (m Manager) metadata() (LaunchAgentMetadata, string, error) {
	if m.AppSupport.Paths.Root == "" {
		return LaunchAgentMetadata{}, "", errors.New("app support paths are required")
	}
	metadata, err := NewLaunchAgentMetadata(m.AppSupport, m.DaemonPath)
	if err != nil {
		return LaunchAgentMetadata{}, "", err
	}
	plistPath, err := LaunchAgentPlistPath()
	if err != nil {
		return LaunchAgentMetadata{}, "", err
	}
	return metadata, plistPath, nil
}

func (m Manager) runner() Runner {
	if m.Runner != nil {
		return m.Runner
	}
	return ExecRunner{}
}

func (m Manager) uid() int {
	if m.UID != nil {
		return m.UID()
	}
	return os.Getuid()
}

func (m Manager) userDomain() string {
	return "gui/" + strconv.Itoa(m.uid())
}

func (m Manager) serviceTarget() string {
	return m.userDomain() + "/" + Label
}

func LaunchAgentPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", PlistName), nil
}

func ResolveDaemonPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if fromEnv := os.Getenv("BROWSERLAB_DAEMON_PATH"); fromEnv != "" {
		return fromEnv, nil
	}
	executable, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "browserlabd")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}
	if fromPath, err := exec.LookPath("browserlabd"); err == nil {
		return fromPath, nil
	}
	return "", errors.New("daemon path is required; pass --daemon-path or set BROWSERLAB_DAEMON_PATH")
}

func writeStringKey(b *bytes.Buffer, key string, value string) {
	fmt.Fprintf(b, "\t<key>%s</key>\n\t<string>%s</string>\n", xmlEscape(key), xmlEscape(value))
}

func writeStringArrayKey(b *bytes.Buffer, key string, values []string) {
	fmt.Fprintf(b, "\t<key>%s</key>\n\t<array>\n", xmlEscape(key))
	for _, value := range values {
		fmt.Fprintf(b, "\t\t<string>%s</string>\n", xmlEscape(value))
	}
	b.WriteString("\t</array>\n")
}

func writeStringDictKey(b *bytes.Buffer, key string, values map[string]string) {
	fmt.Fprintf(b, "\t<key>%s</key>\n\t<dict>\n", xmlEscape(key))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		writeStringKey(b, key, values[key])
	}
	b.WriteString("\t</dict>\n")
}

func writeBoolKey(b *bytes.Buffer, key string, value bool) {
	fmt.Fprintf(b, "\t<key>%s</key>\n", xmlEscape(key))
	if value {
		b.WriteString("\t<true/>\n")
		return
	}
	b.WriteString("\t<false/>\n")
}

func xmlEscape(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}
