package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"dvpl/internal/codec"
	"dvpl/internal/core"
	"dvpl/internal/tui"
)

type config struct {
	mode                                             core.Mode
	input, output                                    string
	level, workers                                   int
	filter, ignore, ignoreCompress                   []string
	forced, trustCRC, deleteOriginal, quiet, verbose bool
	explicitOutput                                   bool
}

func parseArgs(args []string, getenv func(string) string) (config, error) {
	c := config{level: 2}
	command := ""
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "compress", "decompress", "c", "d", "encrypt", "decrypt":
			command = strings.ToLower(args[0])
			args = args[1:]
		}
	}
	flags := flag.NewFlagSet("dvpl", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("o", "", "output path")
	level := flags.Int("level", 2, "compression level: 0, 1, or 2")
	forced := flags.Bool("f", false, "force compression")
	filter := flags.String("filter", "", "include patterns, comma separated")
	ignore := flags.String("ignore", "", "skip patterns, comma separated")
	ignoreCompress := flags.String("ignore-compress", "", "store matching files without compression")
	workers := flags.Int("workers", 0, "number of workers")
	trustCRC := flags.Bool("trust-crc", false, "skip CRC verification")
	deleteOriginal := flags.Bool("delete-original", false, "delete source after successful conversion")
	quiet := flags.Bool("q", false, "suppress per-file output")
	verbose := flags.Bool("v", false, "verbose output")
	if err := flags.Parse(args); err != nil {
		return c, err
	}
	seen := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	pos := flags.Args()
	if len(pos) == 0 {
		return c, errors.New("input is required")
	}
	if len(pos) > 2 {
		return c, errors.New("too many arguments")
	}
	if len(pos) != 1 {
		return c, errors.New("expected one input path")
	}
	c.input = pos[0]
	switch command {
	case "", "compress", "c", "encrypt":
		c.mode = core.Compress
		if command == "" && strings.HasSuffix(strings.ToLower(c.input), ".dvpl") {
			c.mode = core.Decompress
		}
	case "decompress", "d", "decrypt":
		c.mode = core.Decompress
	default:
		return c, fmt.Errorf("unknown command %q", command)
	}
	if !seen["level"] && getenv("DVPL_LEVEL") != "" {
		n, err := strconv.Atoi(getenv("DVPL_LEVEL"))
		if err != nil || n < 0 || n > 2 {
			return c, errors.New("DVPL_LEVEL must be 0, 1, or 2")
		}
		*level = n
	}
	if !seen["workers"] && getenv("DVPL_WORKERS") != "" {
		n, err := strconv.Atoi(getenv("DVPL_WORKERS"))
		if err != nil || n <= 0 {
			return c, errors.New("DVPL_WORKERS must be a positive integer")
		}
		*workers = n
	}
	if *level < 0 || *level > 2 {
		return c, errors.New("-level must be 0, 1, or 2")
	}
	if *workers < 0 {
		return c, errors.New("-workers must not be negative")
	}
	c.output, c.explicitOutput = *output, seen["o"]
	c.level, c.workers = *level, *workers
	c.forced, c.trustCRC, c.deleteOriginal = *forced, *trustCRC, *deleteOriginal
	c.quiet, c.verbose = *quiet, *verbose
	c.filter, c.ignore, c.ignoreCompress = splitList(*filter), splitList(*ignore), splitList(*ignoreCompress)
	return c, nil
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// kindForLevel is the sole CLI-level to codec mapping; footer types are separate.
func kindForLevel(level int) codec.Kind {
	switch level {
	case 0:
		return codec.None
	case 1:
		return codec.LZ4
	default:
		return codec.LZ4HC
	}
}

func run(args []string, getenv func(string) string, stdout, stderr *os.File) int {
	interactive := false
	tuiArgs := args
	if len(args) > 0 && args[0] == "tui" {
		interactive = true
		tuiArgs = args[1:]
	} else if len(args) == 0 {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			interactive = true
		}
	} else if args[0] == "--lang" || strings.HasPrefix(args[0], "--lang=") {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			interactive = true
		}
	}
	if interactive {
		uiLang, err := parseTUILanguage(tuiArgs, getenv)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := tui.Run(uiLang); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	}
	c, err := parseArgs(args, getenv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if info, e := os.Lstat(c.input); e == nil && info.Mode()&os.ModeSymlink != 0 {
		if !c.quiet {
			fmt.Fprintf(stdout, "skip: %s (symlink)\n", c.input)
		}
		fmt.Fprintln(stdout, "Summary: done=0 skipped=1 failed=0 bytes-in=0 bytes-out=0")
		return 0
	}
	if c.trustCRC {
		fmt.Fprintln(stderr, "warning: CRC validation is disabled")
	}
	if c.explicitOutput {
		absIn, _ := filepath.Abs(c.input)
		absOut, _ := filepath.Abs(c.output)
		if rel, e := filepath.Rel(absIn, absOut); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			fmt.Fprintln(stderr, "warning: output is inside input")
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	events := make(chan core.Event)
	job := core.Job{Mode: c.mode, Input: c.input, Output: c.output, Workers: c.workers, Kind: kindForLevel(c.level), Filter: c.filter, Ignore: c.ignore, IgnoreCompress: c.ignoreCompress, Forced: c.forced, TrustCRC: c.trustCRC, DeleteOriginal: c.deleteOriginal}
	done := make(chan error, 1)
	go func() { done <- core.Run(ctx, job, events) }()
	var summary core.Summary
	for ev := range events {
		if ev.Type == core.Finished {
			summary = ev.Summary
		}
		if !c.quiet && (ev.Type == core.FileDone || ev.Type == core.FileSkipped || ev.Type == core.FileError) {
			if ev.Type == core.FileError {
				fmt.Fprintf(stderr, "error: %s: %v\n", ev.Path, ev.Err)
			} else if ev.Type == core.FileSkipped {
				fmt.Fprintf(stdout, "skip: %s\n", ev.Path)
			} else if c.verbose {
				fmt.Fprintf(stdout, "done: %s -> %s (%d -> %d bytes)\n", ev.Path, ev.OutPath, ev.BytesIn, ev.BytesOut)
			} else {
				fmt.Fprintf(stdout, "done: %s\n", ev.Path)
			}
		}
	}
	err = <-done
	if ctx.Err() != nil {
		return 130
	}
	if err != nil && !errors.Is(err, core.ErrFilesFailed) {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "Summary: done=%d skipped=%d failed=%d bytes-in=%d bytes-out=%d\n", summary.Done, summary.Skipped, summary.Failed, summary.BytesIn, summary.BytesOut)
	if errors.Is(err, core.ErrFilesFailed) {
		return 1
	}
	return 0
}

func parseTUILanguage(args []string, getenv func(string) string) (string, error) {
	fs := flag.NewFlagSet("dvpl tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	value := fs.String("lang", "", "interface language: vi or en")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if len(fs.Args()) != 0 {
		return "", errors.New("unexpected arguments for TUI")
	}
	selected := strings.TrimSpace(*value)
	if selected == "" {
		selected = strings.TrimSpace(getenv("DVPL_LANG"))
	}
	if selected == "" {
		selected = "vi"
	}
	if selected != "vi" && selected != "en" {
		return "", errors.New("--lang and DVPL_LANG must be vi or en")
	}
	return selected, nil
}
func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr)) }
