package group

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"time"
	"unsafe"
)

// returns the function pointer
// it is unique for each function
func fptr(r func() error) uintptr {
	return *(*uintptr)(unsafe.Pointer(&r))
}

func cond[T any](cond bool, t, f T) T {
	if cond {
		return t
	}
	return f
}

func filter[T any](s []T, f func(T) bool) []T {
	r := make([]T, 0, len(s)/2)
	for _, v := range s {
		if f(v) {
			r = append(r, v)
		}
	}
	return r
}

func groupMonitor(ctx context.Context, method, prefix string, start time.Time, log bool, err error) {
	if log {
		slog.InfoContext(ctx, fmt.Sprintf("[Group %s] group %s done", method, prefix), slog.Duration("time_to_go", time.Since(start)))
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("[Group %s] group %s failed", method, prefix), slog.String("err", err.Error()))
		}
	}
}

func funcMonitor(ctx context.Context, method, prefix, name string, start time.Time, log bool, errC chan error, err error) {
	if log {
		slog.InfoContext(ctx, fmt.Sprintf("[Group %s] group %s: %s done", method, prefix, name), slog.Duration("time_to_go", time.Since(start)))
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("[Group %s] group %s: %s failed", method, prefix, name), slog.String("err", err.Error()))
		}
	}
	if errC != nil && err != nil {
		errC <- fmt.Errorf("%s failed: %w", name, err)
	}
}

func funcName(f any) string {
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		return "<not a func>"
	}
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return "<unknown>"
	}
	return fn.Name()
}
