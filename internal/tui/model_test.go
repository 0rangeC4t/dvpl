package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dvpl/internal/core"
	"dvpl/internal/tui/lang"
	tea "github.com/charmbracelet/bubbletea"
)

func TestFormBuildsCoreJob(t *testing.T) {
	m := NewModel()
	m.inputs[0].SetValue("source")
	m.inputs[1].SetValue("destination")
	m.level = 1
	m.inputs[2].SetValue("6")
	m.mode = core.Decompress
	m.forced, m.deleteOriginal, m.trustCRC = true, true, true
	m.start()
	if m.screen != progressScreen {
		t.Fatalf("screen=%v, want progress", m.screen)
	}
	if m.job.Mode != core.Decompress || m.job.Input != "source" || m.job.Output != "destination" || m.job.Workers != 6 || m.job.Kind != 1 || !m.job.Forced || !m.job.DeleteOriginal || !m.job.TrustCRC {
		t.Fatalf("unexpected core job: %+v", m.job)
	}
	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
}

func TestFailureListsKeepRecentWindowAndFullResult(t *testing.T) {
	m := NewModel()
	m.screen = progressScreen
	for i := 1; i <= 510; i++ {
		m.addFailure(fmt.Sprintf("file-%03d: CRC mismatch", i))
	}
	progressLines := m.errorLines()
	if len(m.recentFailures) != 500 || m.failureCount != 510 {
		t.Fatalf("recent=%d total=%d", len(m.recentFailures), m.failureCount)
	}
	if progressLines[0] != "... và 10 lỗi khác" || !strings.Contains(progressLines[len(progressLines)-1], "file-510") {
		t.Fatalf("unexpected progress error window: first=%q last=%q", progressLines[0], progressLines[len(progressLines)-1])
	}
	m.screen = resultScreen
	if lines := m.errorLines(); len(lines) != 510 || !strings.Contains(lines[0], "file-001") || !strings.Contains(lines[509], "file-510") {
		t.Fatalf("result should retain all errors; got %d", len(lines))
	}
}

func TestSaveFailures(t *testing.T) {
	m := NewModel()
	m.addFailure("asset.dvpl: unsupported type 3")
	path := filepath.Join(t.TempDir(), "errors.log")
	if got := m.saveFailures(path); !strings.Contains(got, "Đã lưu 1 lỗi") {
		t.Fatalf("save status=%q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "asset.dvpl: unsupported type 3\n" {
		t.Fatalf("saved errors=%q", data)
	}
}

func TestFormRejectsInvalidOptions(t *testing.T) {
	m := NewModel()
	m.inputs[0].SetValue("source")
	m.inputs[2].SetValue("65")
	m.start()
	if m.screen != formScreen || m.formError == "" {
		t.Fatal("invalid workers should keep form open with an error")
	}
}

func TestLevelKeysAndCoreOption(t *testing.T) {
	lang.Set("vi")
	t.Cleanup(func() { lang.Set("vi") })
	m := NewModel()
	if m.level != 2 {
		t.Fatalf("default level=%d, want 2", m.level)
	}
	m.focus = 3
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyLeft})
	if m.level != 1 {
		t.Fatalf("left level=%d, want 1", m.level)
	}
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	if m.level != 0 {
		t.Fatalf("typed level=%d, want 0", m.level)
	}
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if m.level != 1 {
		t.Fatalf("plus level=%d, want 1", m.level)
	}
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.level != 2 || !strings.Contains(m.formView(), "< 2 (HC) >") {
		t.Fatalf("level view/value mismatch: %q", m.formView())
	}
	m.inputs[0].SetValue("source")
	m.inputs[2].SetValue("4")
	m.start()
	if m.job.Kind != 2 {
		t.Fatalf("core compression kind=%d, want 2", m.job.Kind)
	}
	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
}

func TestLanguageCanSwitchFromForm(t *testing.T) {
	lang.Set("vi")
	t.Cleanup(func() { lang.Set("vi") })
	m := NewModel()
	m.focus = 0
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lang.Current() != "en" || !strings.Contains(m.formView(), "Mode") {
		t.Fatalf("language did not switch to English: %q", m.formView())
	}
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lang.Current() != "vi" || !strings.Contains(m.formView(), "Chế độ") {
		t.Fatalf("language did not switch back to Vietnamese: %q", m.formView())
	}
}

func TestWorkersDefaultInputValidationAndCoreOption(t *testing.T) {
	m := NewModel()
	wantDefault := min(64, max(1, runtime.NumCPU()))
	if got := m.inputs[2].Value(); got != fmt.Sprint(wantDefault) {
		t.Fatalf("default workers=%q, want %d", got, wantDefault)
	}
	m.focus = 4
	m.inputs[2].SetValue("")
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.formError == "" {
		t.Fatal("invalid worker input should show an immediate error")
	}
	m.inputs[2].SetValue("7")
	if !m.validateWorkers() {
		t.Fatal("workers=7 should be valid")
	}
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.inputs[2].Value(); got != "8" {
		t.Fatalf("workers plus key=%q, want 8", got)
	}
	m.updateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if got := m.inputs[2].Value(); got != "7" {
		t.Fatalf("workers minus key=%q, want 7", got)
	}
	m.inputs[0].SetValue("source")
	m.start()
	if m.screen != progressScreen || m.job.Workers != 7 {
		t.Fatalf("screen=%d job workers=%d", m.screen, m.job.Workers)
	}
	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{{"1", true}, {"64", true}, {"0", false}, {"65", false}, {"2.5", false}, {"abc", false}} {
		m.inputs[2].SetValue(tc.value)
		if got := m.validateWorkers(); got != tc.valid {
			t.Errorf("workers %q valid=%v, want %v", tc.value, got, tc.valid)
		}
	}
}

func TestProgressAndHumanBytes(t *testing.T) {
	m := Model{total: 4, processed: 3}
	if got := m.percent(); got != .75 {
		t.Fatalf("percent=%v, want .75", got)
	}
	if got := humanBytes(2048); got != "2.0 KiB" {
		t.Fatalf("humanBytes=%q", got)
	}
	_ = humanBytes(int64(^uint64(0) >> 1))
}

func TestInvalidFocusIsResetBeforeUpdate(t *testing.T) {
	m := NewModel()
	m.focus = -1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := updated.(Model).focus; got < 0 || got >= 9 {
		t.Fatalf("focus=%d outside selectable range", got)
	}
}

func TestUpdateBoundsAcrossScreensAndKeys(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
		{Type: tea.KeyTab},
		{Type: tea.KeyShiftTab},
		{Type: tea.KeyEnter},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}
	screens := []screen{formScreen, progressScreen, resultScreen}
	for _, screen := range screens {
		t.Run(fmt.Sprintf("screen-%d", screen), func(t *testing.T) {
			m := NewModel()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m = updated.(Model)
			for _, key := range keys {
				m.screen = screen
				updated, _ = m.Update(key)
				m = updated.(Model)
			}
		})
	}
}

func TestZeroModelWindowSizeAndKeysDoNotPanic(t *testing.T) {
	var m Model
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	for _, key := range []tea.KeyMsg{{Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeyLeft}, {Type: tea.KeyRight}, {Type: tea.KeyTab}, {Type: tea.KeyShiftTab}, {Type: tea.KeyEnter}, {Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'q'}}} {
		updated, _ = m.Update(key)
		m = updated.(Model)
	}
}
