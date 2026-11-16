package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"dvpl/internal/tui/lang"
	tea "github.com/charmbracelet/bubbletea"
)

type browserEntry struct {
	name  string
	path  string
	isDir bool
}

type fileBrowser struct {
	path    string
	entries []browserEntry
	index   int
	target  int
}

func listBrowserEntries(path, goos string) ([]browserEntry, error) {
	if goos == "windows" && (path == "" || path == string(filepath.Separator)) {
		return windowsDriveEntries(os.Stat), nil
	}
	items, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]browserEntry, 0, len(items))
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !item.IsDir() && !item.Type().IsRegular() {
			continue
		}
		entries = append(entries, browserEntry{name: item.Name(), path: filepath.Join(path, item.Name()), isDir: item.IsDir()})
	}
	return entries, nil
}

func windowsDriveEntries(stat func(string) (os.FileInfo, error)) []browserEntry {
	var drives []browserEntry
	for drive := 'A'; drive <= 'Z'; drive++ {
		name := string(drive) + ":\\"
		if _, err := stat(name); err == nil {
			drives = append(drives, browserEntry{name: name, path: name, isDir: true})
		}
	}
	return drives
}

func isWindowsVolumeRoot(path string) bool {
	return len(path) == 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func (m *Model) openBrowser(target int) {
	path := strings.TrimSpace(m.inputs[target].Value())
	if path == "" {
		path, _ = os.Getwd()
	} else if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}
	b := &fileBrowser{path: path, target: target}
	m.browser = b
	b.read()
}

func (b *fileBrowser) read() {
	entries, err := listBrowserEntries(b.path, runtime.GOOS)
	b.entries = nil
	if err == nil {
		b.entries = entries
	}
	if b.index >= len(b.entries) {
		b.index = max(0, len(b.entries)-1)
	}
}

func (m *Model) updateBrowserKey(msg tea.KeyMsg) {
	b := m.browser
	switch msg.String() {
	case "esc":
		m.browser = nil
	case "up":
		if b.index > 0 {
			b.index--
		}
	case "down":
		if b.index+1 < len(b.entries) {
			b.index++
		}
	case "backspace":
		parent := filepath.Dir(b.path)
		if runtime.GOOS == "windows" && isWindowsVolumeRoot(b.path) && strings.EqualFold(parent, b.path) {
			b.path, b.index = "", 0
			b.read()
		} else if parent != b.path {
			b.path, b.index = parent, 0
			b.read()
		}
	case "enter":
		if b.index < len(b.entries) {
			e := b.entries[b.index]
			if e.isDir {
				b.path, b.index = e.path, 0
				b.read()
			} else {
				m.inputs[b.target].SetValue(e.path)
				m.browser = nil
			}
		}
	case " ", "s":
		m.inputs[b.target].SetValue(b.path)
		m.browser = nil
	}
}

func (m Model) browserView() string {
	b := m.browser
	lines := []string{lang.T("browser_title", b.path)}
	if len(b.entries) == 0 {
		lines = append(lines, lang.T("browser_empty"))
	}
	for i, e := range b.entries {
		mark, suffix := "  ", ""
		if i == b.index {
			mark = "> "
		}
		if e.isDir {
			suffix = string(filepath.Separator)
		}
		lines = append(lines, mark+e.name+suffix)
	}
	lines = append(lines, lang.T("browser_help"))
	return strings.Join(lines, "\n")
}
