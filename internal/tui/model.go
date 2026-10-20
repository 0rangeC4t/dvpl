// Package tui provides an interactive terminal interface for DVPL jobs.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dvpl/internal/codec"
	"dvpl/internal/core"
	"dvpl/internal/tui/lang"

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
	inputs                           [3]textinput.Model // source, output, workers
	level                            int
	locale                           string
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
	formError                        string
	width                            int
	height                           int
}

func NewModel() Model {
	inputs := [3]textinput.Model{}
	placeholders := []string{lang.T("source_placeholder"), lang.T("destination_placeholder"), lang.T("workers_placeholder")}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = placeholders[i]
		if i == 0 {
			inputs[i].Focus()
		}
	}
	workers := min(64, max(1, runtime.NumCPU()))
	inputs[2].SetValue(strconv.Itoa(workers))
	return Model{initialized: true, inputs: inputs, level: 2, locale: lang.Current(), progress: progress.New(progress.WithDefaultGradient()), errorViewport: viewport.New(72, 6), width: 80}
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
		if msg.String() == "ctrl+c" || (msg.String() == "q" && !(m.screen == formScreen && isTextFocus(m.focus))) {
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
					m.addFailure(lang.T("failure_line", ev.Path, ev.Err))
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
	if m.screen == formScreen {
		if idx, ok := inputIndex(m.focus); ok {
			var cmd tea.Cmd
			m.inputs[idx], cmd = m.inputs[idx].Update(msg)
			if m.focus == 4 {
				m.validateWorkers()
			}
			return m, cmd
		}
	}
	return m, nil
}

func inputIndex(focus int) (int, bool) {
	switch focus {
	case 1:
		return 0, true
	case 2:
		return 1, true
	case 4:
		return 2, true
	default:
		return 0, false
	}
}

func isTextFocus(focus int) bool { return focus == 1 || focus == 2 || focus == 4 }

func (m *Model) updateFormKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if strings.EqualFold(key, "l") && !isTextFocus(m.focus) {
		m.locale = lang.Other()
		lang.Set(m.locale)
		m.inputs[0].Placeholder = lang.T("source_placeholder")
		m.inputs[1].Placeholder = lang.T("destination_placeholder")
		m.inputs[2].Placeholder = lang.T("workers_placeholder")
		return nil
	}
	if m.focus == 3 {
		switch key {
		case "left", "-":
			m.level = (m.level + 2) % 3
			return nil
		case "right", "+":
			m.level = (m.level + 1) % 3
			return nil
		case "0", "1", "2":
			m.level = int(key[0] - '0')
			return nil
		}
	}
	if m.focus == 4 {
		switch key {
		case "left", "-":
			m.adjustWorkers(-1)
			m.refocusInput()
			return nil
		case "right", "+":
			m.adjustWorkers(1)
			m.refocusInput()
			return nil
		}
	}
	if isTextFocus(m.focus) && key != "tab" && key != "down" && key != "up" && key != "shift+tab" && key != "enter" && key != "esc" {
		idx, _ := inputIndex(m.focus)
		var cmd tea.Cmd
		m.inputs[idx], cmd = m.inputs[idx].Update(msg)
		if m.focus == 4 {
			m.validateWorkers()
		}
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
	if idx, ok := inputIndex(m.focus); ok {
		return m.inputs[idx].Focus()
	}
	return nil
}

func (m *Model) refocusInput() {
	if idx, ok := inputIndex(m.focus); ok {
		m.inputs[idx].Focus()
	}
}

func (m *Model) adjustWorkers(delta int) {
	current, err := strconv.Atoi(strings.TrimSpace(m.inputs[2].Value()))
	if err != nil {
		current = min(64, max(1, runtime.NumCPU()))
	}
	current = min(64, max(1, current+delta))
	m.inputs[2].SetValue(strconv.Itoa(current))
	m.validateWorkers()
}

func (m *Model) validateWorkers() bool {
	workers, err := strconv.Atoi(strings.TrimSpace(m.inputs[2].Value()))
	if err != nil || workers < 1 || workers > 64 {
		m.formError = "workers_invalid"
		return false
	}
	m.formError = ""
	return true
}

func (m *Model) start() {
	m.total, m.processed = 0, 0
	m.summary = core.Summary{}
	m.failures = nil
	m.recentFailures, m.failureCount, m.saveMessage = nil, 0, ""
	m.err = nil
	m.formError = ""
	source := strings.TrimSpace(m.inputs[0].Value())
	if source == "" {
		m.formError = "source_required"
		return
	}
	if m.level < 0 || m.level > 2 {
		m.formError = "level_invalid"
		return
	}
	workers, err := strconv.Atoi(strings.TrimSpace(m.inputs[2].Value()))
	if err != nil || workers < 1 || workers > 64 {
		m.formError = "workers_invalid"
		return
	}
	kinds := [...]codec.Kind{codec.None, codec.LZ4, codec.LZ4HC}
	m.job = core.Job{Mode: m.mode, Input: source, Output: strings.TrimSpace(m.inputs[1].Value()), Workers: workers, Kind: kinds[m.level], Forced: m.forced, DeleteOriginal: m.deleteOriginal, TrustCRC: m.trustCRC}
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
			lines = append(lines, lang.T("dropped_errors", dropped))
		}
		lines = append(lines, m.recentFailures...)
	}
	if len(lines) == 0 {
		lines = []string{lang.T("empty_errors")}
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
		return lang.T("save_error", path, err)
	}
	return lang.T("save_success", len(m.failures), path)
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
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D5FFF")).Render(lang.T("app_title"))
	var body string
	switch m.screen {
	case formScreen:
		body = m.formView()
	case progressScreen:
		elapsed := time.Since(m.started)
		rate := float64(m.summary.Done) / max(elapsed.Seconds(), 0.001)
		body = fmt.Sprintf("%s\n\n%s\n%s\n%s\n%s\n\n%s\n%s\n\n%s", lang.T("progress_title"), m.progress.View(), lang.T("counts", m.summary.Done, m.summary.Skipped, m.summary.Failed, m.processed, m.total), lang.T("rate", rate), lang.T("elapsed", elapsed.Round(time.Second)), lang.T("errors"), m.errorBox(), lang.T("progress_help"))
	case resultScreen:
		body = fmt.Sprintf("%s\n\n%s\n%s\n%s\n%s", lang.T("result_title"), lang.T("summary"), lang.T("counts", m.summary.Done, m.summary.Skipped, m.summary.Failed, m.processed, m.total), lang.T("bytes_in", humanBytes(m.summary.BytesIn)), lang.T("bytes_out", humanBytes(m.summary.BytesOut)))
		if m.err != nil && !errors.Is(m.err, core.ErrFilesFailed) {
			body += "\n" + lang.T("error_prefix", m.err)
		}
		body += "\n\n" + lang.T("errors") + "\n" + m.errorBox()
		if m.saveMessage != "" {
			body += "\n" + m.saveMessage
		}
		body += "\n\n" + lang.T("save_help")
	}
	return title + "\n\n" + body
}

func (m Model) formView() string {
	items := []string{lang.T("mode"), lang.T("source"), lang.T("destination"), lang.T("level"), lang.T("workers"), lang.T("force"), lang.T("delete_original"), lang.T("trust_crc"), lang.T("start")}
	mode := lang.T("compress")
	if m.mode == core.Decompress {
		mode = lang.T("decompress")
	}
	level := min(2, max(0, m.level))
	levelLabel := []string{lang.T("level_none"), lang.T("level_lz4"), lang.T("level_hc")}[level]
	values := []string{mode, m.inputs[0].View(), m.inputs[1].View(), lang.T("level_choice", level, levelLabel), m.inputs[2].View(), check(m.forced), check(m.deleteOriginal), check(m.trustCRC), lang.T("start_symbol")}
	var rows []string
	for i, label := range items {
		style := lipgloss.NewStyle()
		if m.focus == i {
			style = style.Bold(true).Foreground(lipgloss.Color("#7D5FFF"))
		}
		rows = append(rows, style.Render(fmt.Sprintf("%-18s %s", label, values[i])))
	}
	view := strings.Join(rows, "\n") + "\n\n" + lang.T("form_help")
	if m.formError != "" {
		view += "\n\n" + lang.T("error_prefix", lang.T(m.formError))
	}
	return view
}
func check(v bool) string {
	if v {
		return lang.T("force_on")
	}
	return lang.T("force_off")
}
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return lang.T("unit_bytes", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	keys := []string{"unit_kibi", "unit_mebi", "unit_gibi", "unit_tebi", "unit_pebi", "unit_exbi", "unit_zebi", "unit_yobi"}
	if exp >= len(keys) {
		exp = len(keys) - 1
	}
	return lang.T(keys[exp], float64(n)/float64(div))
}

// Run starts the terminal program and waits for any canceled core worker to stop.
func Run(locale string) error {
	lang.Set(locale)
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
