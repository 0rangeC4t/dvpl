// Package tui provides an interactive terminal interface for DVPL jobs.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dvpl/internal/codec"
	"dvpl/internal/core"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen uint8

const (
	formScreen screen = iota
	progressScreen
	resultScreen
)

type Model struct {
	initialized                      bool
	screen                           screen
	mode                             core.Mode
	focus                            int
	inputs                           [4]textinput.Model // source, output, level, workers
	forced, deleteOriginal, trustCRC bool
	job                              core.Job
	cancel                           context.CancelFunc
	events                           <-chan core.Event
	done                             chan error
	total, processed                 int
	summary                          core.Summary
	failures                         []string
	recentFailures                   []string
	failureCount                     int
	errorViewport                    viewport.Model
	saveMessage                      string
	started                          time.Time
	progress                         progress.Model
	err                              error
	width                            int
	height                           int
}

func NewModel() Model {
	inputs := [4]textinput.Model{}
	placeholders := []string{"Đường dẫn nguồn", "Thư mục đích (để trống = cạnh nguồn)", "2", "0 = tự chọn"}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = placeholders[i]
		if i == 0 {
			inputs[i].Focus()
		}
	}
	inputs[2].SetValue("2")
	inputs[3].SetValue("0")
	return Model{initialized: true, inputs: inputs, progress: progress.New(progress.WithDefaultGradient()), errorViewport: viewport.New(72, 6), width: 80}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

type eventBatch []core.Event
type finishedMsg struct{ err error }
type tickMsg time.Time

func readEvent(ch <-chan core.Event, done chan error) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			err := <-done
			done <- err
			return finishedMsg{err}
		}
		batch := []core.Event{ev}
		for len(batch) < 128 {
			select {
			case next, open := <-ch:
				if !open {
					return eventBatch(batch)
				}
				batch = append(batch, next)
			default:
				return eventBatch(batch)
			}
		}
		return eventBatch(batch)
	}
}

func tick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.initialized {
		m = NewModel()
	}
	if m.focus < 0 || m.focus >= 9 {
		m.focus = 0
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = max(10, msg.Width-8)
		m.resizeErrorViewport()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && !(m.screen == formScreen && m.focus >= 1 && m.focus <= 4)) {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.screen == formScreen {
			return m, m.updateFormKey(msg)
		}
		if m.screen == resultScreen && msg.String() == "s" {
			m.saveMessage = m.saveFailures("errors.log")
			return m, nil
		}
		if (m.screen == progressScreen || m.screen == resultScreen) && isScrollKey(msg.String()) {
			var cmd tea.Cmd
			m.errorViewport, cmd = m.errorViewport.Update(msg)
			return m, cmd
		}
		if (msg.String() == "esc" || msg.String() == "enter") && m.screen == resultScreen {
			m.screen = formScreen
			return m, nil
		}
		if msg.String() == "esc" && m.screen == progressScreen {
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil
		}
	case tickMsg:
		if m.screen == progressScreen {
			return m, tick()
		}
	case eventBatch:
		refreshErrors := false
		for _, ev := range msg {
			switch ev.Type {
			case core.Start:
				m.total = ev.Total
			case core.FileDone, core.FileSkipped, core.FileError:
				m.processed++
				m.summary = ev.Summary
				if ev.Type == core.FileError {
					m.addFailure(fmt.Sprintf("%s: %v", ev.Path, ev.Err))
					refreshErrors = true
				}
			case core.Finished:
				m.summary = ev.Summary
				m.screen = resultScreen
				refreshErrors = true
			}
		}
		cmds := []tea.Cmd{readEvent(m.events, m.done)}
		if m.screen == progressScreen {
			cmds = append(cmds, m.progress.SetPercent(m.percent()))
		}
		if refreshErrors {
			m.refreshErrorViewport()
		}
		return m, tea.Batch(cmds...)
	case finishedMsg:
		m.err = msg.err
		m.screen = resultScreen
		m.refreshErrorViewport()
		return m, nil
	case progress.FrameMsg:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.progress.Update(msg)
		m.progress = updated.(progress.Model)
		return m, cmd
	}
	if m.screen == formScreen && m.focus >= 1 && m.focus <= len(m.inputs) {
		var cmd tea.Cmd
		idx := m.focus - 1
		m.inputs[idx], cmd = m.inputs[idx].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) updateFormKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if m.focus >= 1 && m.focus <= 4 && (key == "left" || key == "right") {
		idx := m.focus - 1
		var cmd tea.Cmd
		m.inputs[idx], cmd = m.inputs[idx].Update(msg)
		return cmd
	}
	switch key {
	case "tab", "down":
		m.focus = (m.focus + 1) % 9
	case "shift+tab", "up":
		m.focus = (m.focus + 8) % 9
	case "enter":
		if m.focus >= 1 && m.focus <= 4 {
			m.focus++
		} else {
			switch m.focus {
			case 0:
				if m.mode == core.Compress {
					m.mode = core.Decompress
				} else {
					m.mode = core.Compress
				}
			case 5:
				m.forced = !m.forced
			case 6:
				m.deleteOriginal = !m.deleteOriginal
			case 7:
				m.trustCRC = !m.trustCRC
			case 8:
				m.start()
				if m.screen == progressScreen {
					return tea.Batch(readEvent(m.events, m.done), tick())
				}
			}
		}
	case "left", "right", " ":
		switch m.focus {
		case 0:
			if key == "left" || key == "right" || key == " " {
				if m.mode == core.Compress {
					m.mode = core.Decompress
				} else {
					m.mode = core.Compress
				}
			}
		case 5:
			if key == " " || key == "left" || key == "right" {
				m.forced = !m.forced
			}
		case 6:
			if key == " " || key == "left" || key == "right" {
				m.deleteOriginal = !m.deleteOriginal
			}
		case 7:
			if key == " " || key == "left" || key == "right" {
				m.trustCRC = !m.trustCRC
			}
		}
	case "esc":
		if m.focus > 0 {
			m.focus--
		}
	}
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	if m.focus >= 1 && m.focus <= 4 {
		return m.inputs[m.focus-1].Focus()
	}
	return nil
}

func (m *Model) start() {
	m.total, m.processed = 0, 0
	m.summary = core.Summary{}
	m.failures = nil
	m.recentFailures, m.failureCount, m.saveMessage = nil, 0, ""
	m.recentFailures, m.failureCount, m.saveMessage = nil, 0, ""
	m.err = nil
	source := strings.TrimSpace(m.inputs[0].Value())
	if source == "" {
		m.err = errors.New("cần nhập đường dẫn nguồn")
		return
	}
	var level, workers int
	if _, err := fmt.Sscanf(m.inputs[2].Value(), "%d", &level); err != nil || level < 0 || level > 2 {
		m.err = errors.New("level phải là 0, 1 hoặc 2")
		return
	}
	if _, err := fmt.Sscanf(m.inputs[3].Value(), "%d", &workers); err != nil || workers < 0 {
		m.err = errors.New("workers phải là số không âm")
		return
	}
	kinds := [...]codec.Kind{codec.None, codec.LZ4, codec.LZ4HC}
	m.job = core.Job{Mode: m.mode, Input: source, Output: strings.TrimSpace(m.inputs[1].Value()), Workers: workers, Kind: kinds[level], Forced: m.forced, DeleteOriginal: m.deleteOriginal, TrustCRC: m.trustCRC}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	events := make(chan core.Event, 32)
	done := make(chan error, 1)
	m.events, m.done = events, done
	go func() { done <- core.Run(ctx, m.job, events) }()
	m.started, m.screen, m.err = time.Now(), progressScreen, nil
	m.refreshErrorViewport()
}

func (m Model) percent() float64 {
	if m.total == 0 {
		return 0
	}
	return float64(m.processed) / float64(m.total)
}

func (m *Model) addFailure(line string) {
	m.failures = append(m.failures, line)
	m.failureCount++
	m.recentFailures = append(m.recentFailures, line)
	if len(m.recentFailures) > 500 {
		m.recentFailures = append([]string(nil), m.recentFailures[len(m.recentFailures)-500:]...)
	}
}

func (m *Model) resizeErrorViewport() {
	w := max(10, m.width-8)
	h := max(4, m.height-15)
	if m.screen == resultScreen {
		h = max(4, m.height-13)
	}
	m.errorViewport.Width = w
	m.errorViewport.Height = h
	m.refreshErrorViewport()
}

func (m *Model) refreshErrorViewport() {
	wasAtBottom := m.errorViewport.AtBottom()
	lines := m.errorLines()
	m.errorViewport.SetContent(strings.Join(lines, "\n"))
	if m.screen == progressScreen && wasAtBottom {
		m.errorViewport.GotoBottom()
	}
}

func (m Model) errorLines() []string {
	var lines []string
	if m.screen == resultScreen {
		lines = m.failures
	} else {
		if dropped := m.failureCount - len(m.recentFailures); dropped > 0 {
			lines = append(lines, fmt.Sprintf("... và %d lỗi khác", dropped))
		}
		lines = append(lines, m.recentFailures...)
	}
	if len(lines) == 0 {
		lines = []string{"Không có lỗi."}
	}
	return lines
}

func (m Model) errorBox() string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#E06C75")).Render(m.errorViewport.View())
}

func (m *Model) saveFailures(path string) string {
	data := strings.Join(m.failures, "\n")
	if len(m.failures) > 0 {
		data += "\n"
	}
	path = filepath.Clean(path)
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return fmt.Sprintf("Không lưu được %s: %v", path, err)
	}
	return fmt.Sprintf("Đã lưu %d lỗi vào %s", len(m.failures), path)
}

func isScrollKey(key string) bool {
	switch key {
	case "up", "down", "pgup", "pgdown", "home", "end":
		return true
	default:
		return false
	}
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D5FFF")).Render("DVPL · bộ xử lý tệp")
	var body string
	switch m.screen {
	case formScreen:
		body = m.formView()
	case progressScreen:
		elapsed := time.Since(m.started)
		rate := float64(m.summary.Done) / max(elapsed.Seconds(), 0.001)
		body = fmt.Sprintf("Đang xử lý\n\n%s\nDone: %d   Skipped: %d   Failed: %d   (%d / %d)\nTốc độ: %.1f tệp/s   Thời gian: %s\n\nLỗi\n%s\n\n↑/↓ PgUp/PgDn: cuộn · Esc: hủy · q / Ctrl+C: thoát", m.progress.View(), m.summary.Done, m.summary.Skipped, m.summary.Failed, m.processed, m.total, rate, elapsed.Round(time.Second), m.errorBox())
	case resultScreen:
		body = fmt.Sprintf("Kết quả\n\nSummary\nDone: %d   Skipped: %d   Failed: %d\nBytes in: %s   Bytes out: %s", m.summary.Done, m.summary.Skipped, m.summary.Failed, humanBytes(m.summary.BytesIn), humanBytes(m.summary.BytesOut))
		if m.err != nil && !errors.Is(m.err, core.ErrFilesFailed) {
			body += "\nLỗi: " + m.err.Error()
		}
		body += "\n\nLỗi\n" + m.errorBox()
		if m.saveMessage != "" {
			body += "\n" + m.saveMessage
		}
		body += "\n\n↑/↓ PgUp/PgDn: cuộn · s: lưu errors.log · Enter/Esc: quay lại · q: thoát"
	}
	return title + "\n\n" + body
}

func (m Model) formView() string {
	items := []string{"Chế độ", "Nguồn", "Đích", "Level", "Workers", "Ép nén (-f)", "Xóa file gốc", "Bỏ qua CRC", "Bắt đầu"}
	values := []string{map[core.Mode]string{core.Compress: "Nén", core.Decompress: "Giải nén"}[m.mode], m.inputs[0].View(), m.inputs[1].View(), m.inputs[2].View(), m.inputs[3].View(), check(m.forced), check(m.deleteOriginal), check(m.trustCRC), "▶"}
	var rows []string
	for i, label := range items {
		style := lipgloss.NewStyle()
		if m.focus == i {
			style = style.Bold(true).Foreground(lipgloss.Color("#7D5FFF"))
		}
		rows = append(rows, style.Render(fmt.Sprintf("%-18s %s", label, values[i])))
	}
	view := strings.Join(rows, "\n") + "\n\nTab/mũi tên: chuyển · Enter: chọn · q: thoát"
	if m.err != nil {
		view += "\n\nLỗi: " + m.err.Error()
	}
	return view
}
func (m Model) errString() string {
	if m.err != nil {
		return m.err.Error()
	}
	return ""
}
func check(v bool) string {
	if v {
		return "[x]"
	}
	return "[ ]"
}
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPEZY"
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), units[exp])
}

// Run starts the terminal program and waits for any canceled core worker to stop.
func Run() error {
	m := NewModel()
	program := tea.NewProgram(m, tea.WithAltScreen())
	final, err := program.Run()
	if model, ok := final.(Model); ok && model.cancel != nil {
		model.cancel()
		if model.done != nil {
			<-model.done
		}
	}
	if err != nil {
		return err
	}
	return nil
}
