package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"dvpl/internal/tui/lang"
	tea "github.com/charmbracelet/bubbletea"
)

func isolateUserConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("APPDATA", filepath.Join(dir, "appdata"))
	lang.Set("vi")
	t.Cleanup(func() { lang.Set("vi") })
}

func TestConfigLoadSave(t *testing.T) {
	isolateUserConfig(t)
	path := filepath.Join(t.TempDir(), "settings", "config.json")
	want := userConfig{Source: `C:\assets`, Destination: `D:\out`, Level: 1, Workers: 7, Language: "en"}
	if err := saveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("loaded config=%+v, want %+v", got, want)
	}
	if err := saveConfig(path, userConfig{Source: "updated", Destination: "", Level: 2, Workers: 2, Language: "vi"}); err != nil {
		t.Fatal(err)
	}
	got, err = loadConfig(path)
	if err != nil || got.Source != "updated" {
		t.Fatalf("replacement config=%+v, err=%v", got, err)
	}
	if _, err := loadConfig(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing config should report an error")
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err == nil {
		t.Fatal("malformed config should report an error")
	}
}

func TestNewModelLoadsValidUserConfig(t *testing.T) {
	isolateUserConfig(t)
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := userConfig{Source: "input", Destination: "output", Level: 1, Workers: 7, Language: "en"}
	if err := saveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	got := userConfig{Source: m.inputs[0].Value(), Destination: m.inputs[1].Value(), Level: m.level, Workers: mustWorkers(t, m), Language: m.locale}
	if got != want {
		t.Fatalf("model config=%+v, want %+v", got, want)
	}
	if lang.Current() != "en" {
		t.Fatalf("active language=%q, want en", lang.Current())
	}
}

func TestNewModelConfigFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T)
	}{
		{name: "missing"},
		{name: "malformed", setup: func(t *testing.T) {
			path, err := configPath()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unwritable config directory", setup: func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(base, []byte("block directory creation"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_CONFIG_HOME", base)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateUserConfig(t)
			if tc.setup != nil {
				tc.setup(t)
			}
			lang.Set("en")
			m := NewModel()
			if m.level != 2 {
				t.Errorf("level=%d, want default 2", m.level)
			}
			if got, want := mustWorkers(t, m), min(64, max(1, runtime.NumCPU())); got != want {
				t.Errorf("workers=%d, want default %d", got, want)
			}
			if m.locale != "vi" || lang.Current() != "vi" {
				t.Errorf("locale=%q active=%q, want vi", m.locale, lang.Current())
			}
			m.saveConfig()
		})
	}
}

func mustWorkers(t *testing.T, m Model) int {
	t.Helper()
	workers, err := strconv.Atoi(m.inputs[2].Value())
	if err != nil {
		t.Fatalf("invalid workers value %q: %v", m.inputs[2].Value(), err)
	}
	return workers
}

func TestBrowserListsAndNavigatesDirectories(t *testing.T) {
	isolateUserConfig(t)
	for _, path := range []string{`C:\`, `z:/`} {
		if !isWindowsVolumeRoot(path) {
			t.Errorf("Windows volume root %q was not recognized", path)
		}
	}
	for _, path := range []string{`C:\folder`, `/`, `abc`} {
		if isWindowsVolumeRoot(path) {
			t.Errorf("non-root path %q was recognized as a volume root", path)
		}
	}
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "asset.bin")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := listBrowserEntries(root, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].isDir || entries[0].name != "asset.bin" || !entries[1].isDir {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	m := NewModel()
	m.inputs[0].SetValue(root)
	m.openBrowser(0)
	m.updateBrowserKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.inputs[0].Value(); got != file || m.browser != nil {
		t.Fatalf("file selection=%q browser=%v", got, m.browser)
	}
	m.inputs[1].SetValue(root)
	m.openBrowser(1)
	m.updateBrowserKey(tea.KeyMsg{Type: tea.KeyDown})
	m.updateBrowserKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.browser.path != child {
		t.Fatalf("entered directory=%q, want %q", m.browser.path, child)
	}
	m.updateBrowserKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m.updateBrowserKey(tea.KeyMsg{Type: tea.KeySpace})
	if got := m.inputs[1].Value(); got != root {
		t.Fatalf("directory selection=%q, want %q", got, root)
	}

	var found []string
	drives := windowsDriveEntries(func(path string) (os.FileInfo, error) {
		found = append(found, path)
		if strings.HasPrefix(path, "C:") || strings.HasPrefix(path, "Z:") {
			return nil, nil
		}
		return nil, os.ErrNotExist
	})
	if len(found) != 26 || len(drives) != 2 || drives[0].name != "C:\\" || drives[1].name != "Z:\\" {
		t.Fatalf("drive enumeration: checked=%d drives=%+v", len(found), drives)
	}
}
