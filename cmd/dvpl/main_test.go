package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"dvpl/internal/core"
)

func TestParseArgsFlagsAndEnvironment(t *testing.T) {
	env := map[string]string{"DVPL_LEVEL": "1", "DVPL_WORKERS": "5"}
	getenv := func(k string) string { return env[k] }
	c, err := parseArgs([]string{"compress", "-level", "0", "-workers", "3", "-filter", "*.txt, *.bin", "input"}, getenv)
	if err != nil {
		t.Fatal(err)
	}
	if c.level != 0 || c.workers != 3 || !reflect.DeepEqual(c.filter, []string{"*.txt", "*.bin"}) {
		t.Fatalf("unexpected config: %#v", c)
	}
	if c.mode != core.Compress || c.input != "input" {
		t.Fatalf("unexpected mode/input: %#v", c)
	}
}

func TestParseTUILanguage(t *testing.T) {
	env := func(k string) string {
		if k == "DVPL_LANG" {
			return "en"
		}
		return ""
	}
	if got, err := parseTUILanguage(nil, env); err != nil || got != "en" {
		t.Fatalf("env language=%q err=%v", got, err)
	}
	if got, err := parseTUILanguage([]string{"--lang", "vi"}, env); err != nil || got != "vi" {
		t.Fatalf("flag language=%q err=%v", got, err)
	}
	if _, err := parseTUILanguage([]string{"--lang", "fr"}, env); err == nil {
		t.Fatal("unsupported language should fail")
	}
}

func invokeRun(t *testing.T, args ...string) (int, string) {
	t.Helper()
	logs := t.TempDir()
	stdout, err := os.Create(filepath.Join(logs, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(logs, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	code := run(args, func(string) string { return "" }, stdout, stderr)
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	message, err := os.ReadFile(filepath.Join(logs, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	return code, string(message)
}

func TestRunOutputDirectoryForSingleFile(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	outputDir := filepath.Join(root, "packed")
	decodedDir := filepath.Join(root, "decoded")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(decodedDir, 0755); err != nil {
		t.Fatal(err)
	}
	want := []byte("output directory behavior must preserve every source byte")
	source := filepath.Join(sourceDir, "a.txt")
	if err := os.WriteFile(source, want, 0644); err != nil {
		t.Fatal(err)
	}

	if code, message := invokeRun(t, "compress", "-level", "0", "-o", outputDir, source); code != 0 {
		t.Fatalf("compress returned %d: %s", code, message)
	}
	packed := filepath.Join(outputDir, "a.txt.dvpl")
	if _, err := os.Stat(packed); err != nil {
		t.Fatalf("packed file not in destination directory: %v", err)
	}
	sourceEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sourceEntries) != 1 || sourceEntries[0].Name() != "a.txt" {
		t.Fatalf("unexpected files in source directory: %v", sourceEntries)
	}

	if code, message := invokeRun(t, "decompress", "-o", decodedDir, packed); code != 0 {
		t.Fatalf("decompress returned %d: %s", code, message)
	}
	got, err := os.ReadFile(filepath.Join(decodedDir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded bytes mismatch: got %q, want %q", got, want)
	}
}

func TestRunOutputDirectoryOverwritesOnlyMatchingOutput(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "s")
	destination := filepath.Join(root, "out")
	if err := os.Mkdir(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "a.txt")
	want := []byte("the current source replaces its matching destination")
	if err := os.WriteFile(source, want, 0644); err != nil {
		t.Fatal(err)
	}
	packedPath := filepath.Join(destination, "a.txt.dvpl")
	if err := os.WriteFile(packedPath, []byte("unrelated old contents"), 0644); err != nil {
		t.Fatal(err)
	}
	keepPath := filepath.Join(destination, "keep.dat")
	if err := os.WriteFile(keepPath, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	if code, message := invokeRun(t, "compress", "-level", "0", "-o", destination, source); code != 0 {
		t.Fatalf("compress returned %d: %s", code, message)
	}
	packed, err := os.ReadFile(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(packed, []byte("unrelated old contents")) {
		t.Fatal("matching output was not overwritten")
	}
	kept, err := os.ReadFile(keepPath)
	if err != nil || string(kept) != "keep" {
		t.Fatalf("unrelated destination file changed or disappeared: %q, %v", kept, err)
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected destination entries: %v", entries)
	}
}

func TestRunRejectsFileAsOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	output := filepath.Join(root, "output-file")
	if err := os.WriteFile(source, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("leave unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	code, _ := invokeRun(t, "compress", "-o", output, source)
	if code != 2 {
		t.Fatalf("run returned %d, want configuration exit code 2", code)
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "leave unchanged" {
		t.Fatalf("output file changed: %q, %v", got, err)
	}
}

func TestParseArgsEnvironmentAndInference(t *testing.T) {
	getenv := func(k string) string {
		if k == "DVPL_LEVEL" {
			return "1"
		}
		if k == "DVPL_WORKERS" {
			return "4"
		}
		return ""
	}
	c, err := parseArgs([]string{"thing.DVPL"}, getenv)
	if err != nil {
		t.Fatal(err)
	}
	if c.mode != core.Decompress || c.level != 1 || c.workers != 4 {
		t.Fatalf("unexpected config: %#v", c)
	}
}

func TestParseArgsRejectsBadEnvironment(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"DVPL_LEVEL", "3"}, {"DVPL_WORKERS", "0"}} {
		t.Run(tc.key, func(t *testing.T) {
			_, err := parseArgs([]string{"input"}, func(k string) string {
				if k == tc.key {
					return tc.value
				}
				return ""
			})
			if err == nil {
				t.Fatalf("accepted %s=%s", tc.key, tc.value)
			}
		})
	}
}
