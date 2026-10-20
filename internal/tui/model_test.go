package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dvpl/internal/core"
)

func TestFormBuildsCoreJob(t *testing.T) {
	m := NewModel()
	m.inputs[0].SetValue("source")
	m.inputs[1].SetValue("destination")
	m.inputs[2].SetValue("1")
	m.inputs[3].SetValue("6")
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
	m.inputs[2].SetValue("3")
	m.start()
	if m.screen != formScreen || m.err == nil {
		t.Fatal("invalid level should keep form open with an error")
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
}
