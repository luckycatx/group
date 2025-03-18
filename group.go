package group

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

func groupGo(ctx context.Context, g *errgroup.Group, opts *Options, fs ...func() error) {
	for _, f := range fs {
		g.Go(func() (err error) {
			// ctx check before exec
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// no opts short circuit
			if opts == nil || !opts.WithLog && opts.ErrC == nil {
				return SafeRun(ctx, f)
			}

			if opts.WithLog || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "groupGo", opts.Prefix, funcName(f), start, opts.WithLog, opts.ErrC, err)
				}(time.Now())
			}
			return SafeRun(ctx, f)
		})
	}
}

func groupTryGo(ctx context.Context, g *errgroup.Group, opts *Options, fs ...func() error) bool {
	ok := true
	for _, f := range fs {
		ok = ok && g.TryGo(func() (err error) {
			// ctx check before exec
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// no opts short circuit
			if opts == nil || !opts.WithLog && opts.ErrC == nil {
				return SafeRun(ctx, f)
			}

			if opts.WithLog || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "groupTryGo", opts.Prefix, funcName(f), start, opts.WithLog, opts.ErrC, err)
				}(time.Now())
			}
			return SafeRun(ctx, f)
		})
	}
	return ok
}

func (d depMap) groupGo(ctx context.Context, gtx context.Context, g *errgroup.Group, opts *Options) {
	var sigs = make(map[string]signal, len(d))
	for _, fd := range d {
		// skip anonymous func
		if fd.deps[0] == "" {
			continue
		}
		// self signal
		sigs[fd.deps[0]] = make(signal)
	}

	for r := range d {
		g.Go(func() (err error) {
			// ctx check before exec
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-gtx.Done(): // early-stage err check before dep signal
				return gtx.Err()
			default:
				defer notify(sigs[d[r].deps[0]])
			}

			var depErr error // record dep err
			for i, dep := range d[r].deps {
				if i == 0 {
					continue
				}
				if sigs[dep] == nil {
					return fmt.Errorf("missing dep signal for %s", dep)
				}
				<-sigs[dep] // wait for dep signal
				// ctx check after dep signal
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-gtx.Done():
					// timeout is always fatal
					if errors.Is(gtx.Err(), context.DeadlineExceeded) {
						return gtx.Err()
					}
					// tolerance check
					if _, ok := opts.tol[dep]; !ok {
						return gtx.Err()
					}
					// propagate tolerance & record err
					opts.tol[d[r].deps[0]], depErr = token{}, gtx.Err()
				default: // ctx ok
				}
			}

			if opts.WithLog || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "depMap.groupGo", opts.Prefix, cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), start, opts.WithLog, opts.ErrC, err)
				}(time.Now())
			}
			if err = SafeRun(gtx, d[r].f); err != nil {
				return cond(depErr != nil, fmt.Errorf("%v -> %w", depErr, err), err)
			}
			return depErr
		})
	}
}

func (d depMap) groupTryGo(ctx context.Context, gtx context.Context, g *errgroup.Group, opts *Options) bool {
	var sigs = make(map[string]signal, len(d))
	for _, fd := range d {
		// skip anonymous func
		if fd.deps[0] == "" {
			continue
		}
		// self signal
		sigs[fd.deps[0]] = make(signal)
	}

	ok := true
	for r := range d {
		ok = ok && g.TryGo(func() (err error) {
			// ctx check before exec
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-gtx.Done(): // early-stage err check before dep signal
				return gtx.Err()
			default:
				defer notify(sigs[d[r].deps[0]])
			}

			var depErr error // record dep err
			for i, dep := range d[r].deps {
				if i == 0 {
					continue
				}
				if sigs[dep] == nil {
					return fmt.Errorf("missing dep signal for %s", dep)
				}
				<-sigs[dep] // wait for dep signal
				// ctx check after dep signal
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-gtx.Done():
					// timeout is always fatal
					if errors.Is(gtx.Err(), context.DeadlineExceeded) {
						return gtx.Err()
					}
					// tolerance check
					if _, ok := opts.tol[dep]; !ok {
						return gtx.Err()
					}
					// propagate tolerance & record err
					opts.tol[d[r].deps[0]], depErr = token{}, gtx.Err()
				default: // ctx ok
				}
			}

			if opts.WithLog || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "depMap.groupTryGo", opts.Prefix, cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), start, opts.WithLog, opts.ErrC, err)
				}(time.Now())
			}
			if err = SafeRun(gtx, d[r].f); err != nil {
				return cond(depErr != nil, fmt.Errorf("%v -> %w", depErr, err), err)
			}
			return depErr
		})
	}
	return ok
}
