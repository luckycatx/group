package benchmark

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/luckycatx/group"
)

// Task represents a unit of work to benchmark
type Task struct {
	ID       int
	Workload int           // cpu intensity
	IO       time.Duration // i/o waiting simulation
}

func genTasks(cnt int, workload int, io time.Duration) []Task {
	tasks := make([]Task, cnt)
	for i := range cnt {
		tasks[i] = Task{
			ID:       i,
			Workload: workload,
			IO:       io,
		}
	}
	return tasks
}

func process(task Task) error {
	res := 0
	for i := range task.Workload {
		res += i
	}
	if task.IO > 0 {
		time.Sleep(task.IO)
	}
	return nil
}

func setupFuncs(tasks []Task) []func() error {
	fs := make([]func() error, len(tasks))
	for i, task := range tasks {
		fs[i] = func() error { return process(task) }
	}
	return fs
}

//= BENCHMARK

var cases = []struct {
	name          string
	cnt, workload int
	io            time.Duration
}{
	{"Tiny", 10, 100, 0 * time.Millisecond},
	{"Small", 100, 1000, 1 * time.Millisecond},
	{"Medium", 1000, 10000, 5 * time.Millisecond},
	{"Large", 10000, 100000, 10 * time.Millisecond},
}

func BenchmarkGo(b *testing.B) {
	// discard log output
	slog.SetDefault(slog.New(slog.DiscardHandler))
	fmt.Println()
	for _, c := range cases {
		b.Run(fmt.Sprintf("%sWorkload", c.name), func(b *testing.B) {
			benchmarkGo(b, genTasks(c.cnt, c.workload, c.io))
		})
		fmt.Println()
	}
}

func benchmarkGo(b *testing.B, tasks []Task) {
	b.Run("StdGoroutine", func(b *testing.B) {
		fs := setupFuncs(tasks)
		runStdGoroutine(b, fs...)
	})

	b.Run("StdErrGroup", func(b *testing.B) {
		fs := setupFuncs(tasks)
		runStdErrGroup(b, fs...)
	})

	b.Run("GroupGo", func(b *testing.B) {
		fs := setupFuncs(tasks)
		runGroupGo(b, nil, fs...)
	})

	b.Run("GroupGoWithOpts", func(b *testing.B) {
		fs := setupFuncs(tasks)
		runGroupGo(b, group.Opts(group.WithTimeout(1*time.Minute), group.WithLog()), fs...)
	})
}

func runStdGoroutine(b *testing.B, fs ...func() error) {
	for b.Loop() {
		var wg sync.WaitGroup
		wg.Add(len(fs))
		for _, f := range fs {
			go func() {
				defer wg.Done()
				_ = f()
			}()
		}
		wg.Wait()
	}
}

func runStdErrGroup(b *testing.B, fs ...func() error) {
	for b.Loop() {
		g, _ := errgroup.WithContext(context.Background())
		for _, f := range fs {
			g.Go(f)
		}
		_ = g.Wait()
	}
}

func runGroupGo(b *testing.B, opts *group.Options, fs ...func() error) {
	for b.Loop() {
		_ = group.Go(context.Background(), opts, fs...)
	}
}

// >> BENCHMARK - DEP

type benchmarkCtx struct {
	a, b, c, d int
}

func (c *benchmarkCtx) A() error {
	c.a = 1
	return nil
}

func (c *benchmarkCtx) B() error {
	c.b = c.a + 1
	return nil
}

func (c *benchmarkCtx) C() error {
	c.c = c.a + 1
	return nil
}

func (c *benchmarkCtx) D() error {
	c.d = c.b + c.c
	return nil
}

func BenchmarkGoDep(b *testing.B) {
	// discard log output
	slog.SetDefault(slog.New(slog.DiscardHandler))
	fmt.Println()
	benchmarkGoDep(b)
	fmt.Println()
}

func benchmarkGoDep(b *testing.B) {
	b.Run("StdErrGroup", func(b *testing.B) {
		runStdErrGroupDep(b, new(benchmarkCtx))
	})

	b.Run("GroupGo", func(b *testing.B) {
		runGroupGoDep(b, new(benchmarkCtx))
	})
}

func runStdErrGroupDep(b *testing.B, c *benchmarkCtx) {
	for b.Loop() {
		g, _ := errgroup.WithContext(context.Background())
		g.Go(c.A)
		_ = g.Wait()
		g, _ = errgroup.WithContext(context.Background())
		g.Go(c.B)
		g.Go(c.C)
		_ = g.Wait()
		g, _ = errgroup.WithContext(context.Background())
		g.Go(c.D)
		_ = g.Wait()
	}
}

func runGroupGoDep(b *testing.B, c *benchmarkCtx) {
	for b.Loop() {
		var opts = group.Opts(group.WithDep)
		_ = group.Go(context.Background(), opts,
			group.MakeRunner(c.A).Name(opts, "a"),
			group.MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
			group.MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
			group.MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))
	}
}
