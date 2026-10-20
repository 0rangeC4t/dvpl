package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dvpl/internal/codec"
	"dvpl/internal/dvpl"
)

var errBadPattern = errors.New("invalid file pattern")

func checkPatterns(groups ...[]string) error {
	for _, patterns := range groups {
		for _, pattern := range patterns {
			if _, err := filepath.Match(pattern, ""); err != nil {
				return fmt.Errorf("%w %q: %v", errBadPattern, pattern, err)
			}
		}
	}
	return nil
}

func listFiles(job Job) ([]string, error) {
	info, err := os.Lstat(job.Input)
	if err != nil {
		return nil, fmt.Errorf("input: %w", err)
	}
	var paths []string
	add := func(path, name string, regular bool) error {
		if strings.HasSuffix(name, ".dvpltmp") || !regular {
			return nil
		}
		if job.Mode == Compress && strings.HasSuffix(name, ".dvpl") {
			return nil
		}
		if job.Mode == Decompress && !strings.HasSuffix(name, ".dvpl") {
			return nil
		}
		paths = append(paths, path)
		return nil
	}
	if info.Mode().IsRegular() {
		entryInfo, err := os.Stat(job.Input)
		if err != nil {
			return nil, err
		}
		if !entryInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("input is not a regular file")
		}
		if err := add(job.Input, filepath.Base(job.Input), true); err != nil {
			return nil, err
		}
	} else if info.IsDir() {
		err = filepath.WalkDir(job.Input, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == job.Input {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			return add(path, d.Name(), d.Type().IsRegular())
		})
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("input must be a regular file or directory")
	}
	selected := paths[:0]
	for _, path := range paths {
		matched := len(job.Filter) == 0
		if !matched {
			matched, err = matches(job.Filter, filepath.Base(path))
		}
		if err != nil {
			return nil, err
		}
		if matched {
			selected = append(selected, path)
		}
	}
	return selected, nil
}

func matches(patterns []string, name string) (bool, error) {
	for _, pattern := range patterns {
		ok, err := filepath.Match(pattern, name)
		if err != nil {
			return false, fmt.Errorf("%w %q: %v", errBadPattern, pattern, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func outputPath(job Job, path string) string {
	name := filepath.Base(path)
	if job.Mode == Compress {
		name += ".dvpl"
	} else {
		name = strings.TrimSuffix(name, ".dvpl")
	}
	if job.Output == "" {
		return filepath.Join(filepath.Dir(path), name)
	}
	inputInfo, err := os.Stat(job.Input)
	if err == nil && inputInfo.IsDir() {
		rel, _ := filepath.Rel(job.Input, path)
		return filepath.Join(job.Output, filepath.Dir(rel), name)
	}
	return filepath.Join(job.Output, name)
}

type fileResult struct {
	out               string
	inBytes, outBytes int64
	skipped           bool
}

func processFile(job Job, path string) (fileResult, error) {
	result := fileResult{out: outputPath(job, path)}
	skip, err := matches(job.Ignore, filepath.Base(path))
	if err != nil {
		return result, err
	}
	if skip {
		result.skipped = true
		return result, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	result.inBytes = int64(len(data))
	var encoded []byte
	if job.Mode == Compress {
		kind := job.Kind
		ignored, err := matches(job.IgnoreCompress, filepath.Base(path))
		if err != nil {
			return result, err
		}
		if ignored {
			kind = codec.None
		}
		encoded, err = dvpl.Pack(data, kind, job.Forced)
	} else {
		encoded, err = dvpl.Unpack(data, job.TrustCRC)
	}
	if err != nil {
		return result, err
	}
	result.outBytes = int64(len(encoded))
	if err := atomicWrite(result.out, encoded); err != nil {
		return result, err
	}
	if job.DeleteOriginal {
		srcInfo, se := os.Stat(path)
		dstInfo, de := os.Stat(result.out)
		if se == nil && de == nil && !os.SameFile(srcInfo, dstInfo) {
			if err := os.Remove(path); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "*.dvpltmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	n, writeErr := tmp.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return errors.Join(writeErr, closeErr)
		}
		return writeErr
	}
	if err := tmp.Sync(); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmpName, 0644)
	tries := 1
	if runtime.GOOS == "windows" {
		tries = 3
	}
	for i := 0; i < tries; i++ {
		err = os.Rename(tmpName, path)
		if err == nil {
			return nil
		}
		if runtime.GOOS != "windows" || i+1 == tries {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}
