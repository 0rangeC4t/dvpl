package core

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dvpl/internal/codec"
)

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func data(n int) []byte { return []byte(strings.Repeat("nested DVPL test payload ", n)) }
func noTemps(t *testing.T, root string) {
	t.Helper()
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".dvpltmp") {
			t.Errorf("temporary file remains: %s", p)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}

func TestCompressDecompressTree(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	out := filepath.Join(root, "packed")
	decoded := filepath.Join(root, "decoded")
	want := map[string][]byte{"one.txt": data(200), filepath.Join("sub", "two.bin"): data(120)}
	for name, b := range want {
		put(t, filepath.Join(src, name), b)
	}
	events := make(chan Event, 16)
	if err := Run(context.Background(), Job{Mode: Compress, Input: src, Output: out, Workers: 4, Kind: codec.LZ4HC}, events); err != nil {
		t.Fatal(err)
	}
	if _, ok := <-events; !ok {
		t.Fatal("missing events")
	}
	for range events {
	}
	events = make(chan Event, 16)
	if err := Run(context.Background(), Job{Mode: Decompress, Input: out, Output: decoded, Workers: 4}, events); err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	for name, b := range want {
		got := read(t, filepath.Join(decoded, name))
		if string(got) != string(b) {
			t.Fatalf("content mismatch for %s", name)
		}
	}
	noTemps(t, root)
}

func TestFilterIgnoreAndIgnoreCompress(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	out := filepath.Join(root, "out")
	put(t, filepath.Join(src, "keep.txt"), data(10))
	put(t, filepath.Join(src, "raw.txt"), data(10))
	put(t, filepath.Join(src, "skip.txt"), data(10))
	put(t, filepath.Join(src, "no.bin"), data(10))
	events := make(chan Event, 16)
	err := Run(context.Background(), Job{Mode: Compress, Input: src, Output: out, Workers: 4, Kind: codec.LZ4, Filter: []string{"*.txt"}, Ignore: []string{"skip.txt"}, IgnoreCompress: []string{"raw.txt"}}, events)
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	if _, e := os.Stat(filepath.Join(out, "no.bin.dvpl")); !os.IsNotExist(e) {
		t.Fatal("filter did not exclude .bin")
	}
	if _, e := os.Stat(filepath.Join(out, "skip.txt.dvpl")); !os.IsNotExist(e) {
		t.Fatal("ignore did not skip file")
	}
	footer := read(t, filepath.Join(out, "raw.txt.dvpl"))
	label := binary.LittleEndian.Uint32(footer[len(footer)-8 : len(footer)-4])
	if label != 0 {
		t.Fatalf("IgnoreCompress type=%d, want 0", label)
	}
	noTemps(t, root)
}

func TestDeleteOriginalBehavior(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source.txt")
	out := filepath.Join(root, "out")
	put(t, src, data(100))
	if err := Run(context.Background(), Job{Mode: Compress, Input: src, Output: out, Kind: codec.LZ4}, nil); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(src); e != nil {
		t.Fatal("default removed original")
	}
	second := filepath.Join(root, "second.txt")
	put(t, second, data(100))
	if err := Run(context.Background(), Job{Mode: Compress, Input: second, Output: out, Kind: codec.LZ4, DeleteOriginal: true}, nil); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(second); !os.IsNotExist(e) {
		t.Fatal("DeleteOriginal did not remove source after success")
	}
	bad := filepath.Join(out, "broken.txt.dvpl")
	put(t, bad, []byte("invalid footer"))
	if err := Run(context.Background(), Job{Mode: Decompress, Input: bad, Output: filepath.Join(root, "decoded"), DeleteOriginal: true}, nil); !errors.Is(err, ErrFilesFailed) {
		t.Fatalf("got %v", err)
	}
	if _, e := os.Stat(bad); e != nil {
		t.Fatal("failed file was removed")
	}
	noTemps(t, root)
}

func TestFileErrorDoesNotStopBatch(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in")
	put(t, filepath.Join(src, "a.txt"), data(50))
	put(t, filepath.Join(src, "b.txt"), data(50))
	put(t, filepath.Join(src, "c.txt"), data(50))
	// An invalid compression kind fails each file; a corrupt source in decompress mode
	// mixed with valid input verifies per-file continuation.
	packed := filepath.Join(root, "packed")
	if err := Run(context.Background(), Job{Mode: Compress, Input: src, Output: packed, Kind: codec.LZ4}, nil); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(packed, "broken.dvpl"), []byte("bad"))
	out := filepath.Join(root, "decoded")
	err := Run(context.Background(), Job{Mode: Decompress, Input: packed, Output: out, Workers: 4}, nil)
	if !errors.Is(err, ErrFilesFailed) {
		t.Fatalf("got %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, e := os.Stat(filepath.Join(out, name)); e != nil {
			t.Errorf("valid file %s did not finish: %v", name, e)
		}
	}
	noTemps(t, root)
}

func TestValidationAndCancellation(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.txt")
	put(t, input, data(100))
	if err := Run(context.Background(), Job{Mode: Compress, Input: input, Filter: []string{"["}}, nil); err == nil || errors.Is(err, ErrFilesFailed) {
		t.Fatalf("bad pattern should be config error: %v", err)
	}
	regular := filepath.Join(root, "regular")
	put(t, regular, nil)
	if err := Run(context.Background(), Job{Mode: Compress, Input: input, Output: regular}, nil); err == nil || errors.Is(err, ErrFilesFailed) {
		t.Fatalf("regular output should be config error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 8)
	go func() {
		for ev := range events {
			if ev.Type == Start {
				cancel()
			}
		}
	}()
	large := filepath.Join(root, "large.bin")
	put(t, large, data(100000))
	err := Run(ctx, Job{Mode: Compress, Input: large, Output: filepath.Join(root, "cancelled"), Workers: 4, Kind: codec.LZ4HC}, events)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	noTemps(t, root)
}

func TestEventsNotDroppedWithSlowReceiver(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in")
	out := filepath.Join(root, "out")
	for i := 0; i < 20; i++ {
		put(t, filepath.Join(src, string(rune('a'+i))+".txt"), data(1))
	}

	events := make(chan Event)
	received := make(chan []Event, 1)
	go func() {
		var got []Event
		for event := range events {
			got = append(got, event)
			time.Sleep(5 * time.Millisecond)
		}
		received <- got
	}()

	if err := Run(context.Background(), Job{Mode: Compress, Input: src, Output: out}, events); err != nil {
		t.Fatal(err)
	}
	got := <-received
	starts, done, finished := 0, 0, 0
	for _, event := range got {
		switch event.Type {
		case Start:
			starts++
			if event.Total != 20 {
				t.Errorf("Start Total=%d, want 20", event.Total)
			}
		case FileDone:
			done++
		case Finished:
			finished++
			if event.Summary.Done != 20 {
				t.Errorf("Finished Summary.Done=%d, want 20", event.Summary.Done)
			}
		}
	}
	if starts != 1 || done != 20 || finished != 1 {
		t.Errorf("got Start=%d FileDone=%d Finished=%d; want 1, 20, 1", starts, done, finished)
	}
	if _, ok := <-events; ok {
		t.Error("events channel is still open")
	}
}
