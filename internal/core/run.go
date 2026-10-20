package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"

	"dvpl/internal/codec"
)

type Mode uint8

const (
	Compress Mode = iota
	Decompress
)

type Job struct {
	Mode                             Mode
	Input, Output                    string
	Workers                          int
	Kind                             codec.Kind
	Filter, Ignore, IgnoreCompress   []string
	Forced, TrustCRC, DeleteOriginal bool
}
type EventType uint8

const (
	Start EventType = iota
	FileDone
	FileSkipped
	FileError
	Finished
)

type Event struct {
	Type              EventType
	Path, OutPath     string
	Err               error
	BytesIn, BytesOut int64
	Total             int
	Summary           Summary
}
type Summary struct {
	Done, Skipped, Failed int
	BytesIn, BytesOut     int64
}

var ErrFilesFailed = errors.New("one or more files failed")

func validateJob(job Job) error {
	if job.Mode != Compress && job.Mode != Decompress {
		return errors.New("invalid mode")
	}
	if job.Input == "" {
		return errors.New("input is required")
	}
	if job.Kind != codec.None && job.Kind != codec.LZ4 && job.Kind != codec.LZ4HC {
		return errors.New("invalid compression kind")
	}
	if err := checkPatterns(job.Filter, job.Ignore, job.IgnoreCompress); err != nil {
		return err
	}
	if job.Output != "" {
		if info, err := os.Stat(job.Output); err == nil && !info.IsDir() {
			return errors.New("output exists and is not a directory")
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("output: %w", err)
		}
	}
	return nil
}

func emit(ctx context.Context, events chan<- Event, event Event) {
	if events == nil {
		return
	}
	select {
	case events <- event:
	case <-ctx.Done():
	}
}

func Run(ctx context.Context, job Job, events chan<- Event) error {
	if events != nil {
		defer close(events)
	}
	if err := validateJob(job); err != nil {
		return err
	}
	paths, err := listFiles(job)
	if err != nil {
		return err
	}
	workers := job.Workers
	if workers <= 0 {
		workers = 2
	}
	cpu := runtime.NumCPU()
	if cpu < 1 {
		cpu = 1
	}
	if workers > cpu {
		workers = cpu
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan string)
	results := make(chan fileOutcome, len(paths))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				res, e := processFile(job, path)
				results <- fileOutcome{path: path, result: res, err: e}
			}
		}()
	}
	aggDone := make(chan Summary, 1)
	go func() {
		var sum Summary
		emit(ctx, events, Event{Type: Start, Total: len(paths)})
		for outcome := range results {
			ev := Event{Path: outcome.path, OutPath: outcome.result.out}
			if outcome.err != nil {
				sum.Failed++
				ev.Type = FileError
				ev.Err = outcome.err
			} else if outcome.result.skipped {
				sum.Skipped++
				ev.Type = FileSkipped
			} else {
				sum.Done++
				sum.BytesIn += outcome.result.inBytes
				sum.BytesOut += outcome.result.outBytes
				ev.Type = FileDone
				ev.BytesIn = outcome.result.inBytes
				ev.BytesOut = outcome.result.outBytes
			}
			ev.Summary = sum
			emit(ctx, events, ev)
		}
		emit(ctx, events, Event{Type: Finished, Summary: sum})
		aggDone <- sum
	}()
	for _, path := range paths {
		if ctx.Err() != nil {
			break
		}
		select {
		case jobs <- path:
		case <-ctx.Done():
			goto dispatched
		}
	}
dispatched:
	close(jobs)
	wg.Wait()
	close(results)
	sum := <-aggDone
	if err := ctx.Err(); err != nil {
		return err
	}
	if sum.Failed > 0 {
		return fmt.Errorf("%w: %d file(s)", ErrFilesFailed, sum.Failed)
	}
	return nil
}

type fileOutcome struct {
	path   string
	result fileResult
	err    error
}
