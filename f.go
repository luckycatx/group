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

func groupTimer(ctx context.Context, method, prefix string, start time.Time) {
	slog.InfoContext(ctx, fmt.Sprintf("[Group %s] group %s done", method, prefix), slog.Duration("time_to_go", time.Since(start)))
}

func funcTimer(ctx context.Context, method, prefix, name string, start time.Time) {
	slog.InfoContext(ctx, fmt.Sprintf("[Group %s] group %s: %s done", method, prefix, name), slog.Duration("time_to_go", time.Since(start)))
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
