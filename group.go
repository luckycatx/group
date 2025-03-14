package group

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

func groupGo(ctx context.Context, g *errgroup.Group, opts *Options, fs ...func() error) {
	for _, f := range fs {
		g.Go(func() error {
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
			if opts.WithLog {
				defer funcTimer(ctx, "groupGo", opts.Prefix, funcName(f), time.Now())
			}
			if err := SafeRun(ctx, f); err != nil {
				fName := funcName(f)
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", fName, err)
				}
				if opts.WithLog {
					slog.ErrorContext(ctx, fmt.Sprintf("[Group groupGo] group %s: %s failed", opts.Prefix, fName), slog.String("err", fmt.Sprintf("%+v", err)))
				}
				return err
			}
			return nil
		})
	}
}

func groupTryGo(ctx context.Context, g *errgroup.Group, opts *Options, fs ...func() error) bool {
	ok := true
	for _, f := range fs {
		ok = ok && g.TryGo(func() error {
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
			if opts.WithLog {
				defer funcTimer(ctx, "groupTryGo", opts.Prefix, funcName(f), time.Now())
			}
			if err := SafeRun(ctx, f); err != nil {
				fName := funcName(f)
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", fName, err)
				}
				if opts.WithLog {
					slog.ErrorContext(ctx, fmt.Sprintf("[Group groupTryGo] group %s: %s failed", opts.Prefix, fName), slog.String("err", fmt.Sprintf("%+v", err)))
				}
				return err
			}
			return nil
		})
	}
	return ok
}

func (d depMap) groupGo(ctx context.Context, gtx context.Context, g *errgroup.Group, opts *Options) {
	var sigs, tols = make(map[string]signal, len(d)), make(map[string]token)
	for _, fd := range d {
		// skip anonymous func
		if fd.deps[0] == "" {
			continue
		}
		// self signal
		dep := trimTol(fd.deps[0])
		sigs[dep] = make(signal)
		if tolerance(fd.deps[0]) {
			// tolerance token
			tols[dep], fd.deps[0] = token{}, dep
		}
	}

	for r := range d {
		g.Go(func() error {
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
					if _, ok := tols[dep]; !ok {
						return gtx.Err()
					}
					tols[d[r].deps[0]] = token{} // propagate tolerance
					depErr = gtx.Err()           // record and continue
				default: // ctx ok
				}
			}
			// no opts short circuit
			if !opts.WithLog && opts.ErrC == nil {
				if err := SafeRun(gtx, d[r].f); err != nil {
					return errors.Join(err, depErr)
				}
				return depErr
			}
			if opts.WithLog {
				defer funcTimer(ctx, "Dep.groupGo", opts.Prefix, cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), time.Now())
			}
			if err := SafeRun(gtx, d[r].f); err != nil {
				fName := cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f))
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", fName, err)
				}
				if opts.WithLog {
					slog.ErrorContext(ctx, fmt.Sprintf("[Group Dep.groupGo] group %s: %s failed", opts.Prefix, fName), slog.String("err", fmt.Sprintf("%+v", err)))
				}
				return errors.Join(err, depErr)
			}
			return depErr
		})
	}
}

func (d depMap) groupTryGo(ctx context.Context, gtx context.Context, g *errgroup.Group, opts *Options) bool {
	var sigs, tols = make(map[string]signal, len(d)), make(map[string]token)
	for _, fd := range d {
		// skip anonymous func
		if fd.deps[0] == "" {
			continue
		}
		// self signal
		dep := trimTol(fd.deps[0])
		sigs[dep] = make(signal)
		if tolerance(fd.deps[0]) {
			// tolerance token
			tols[dep], fd.deps[0] = token{}, dep
		}
	}

	ok := true
	for r := range d {
		ok = ok && g.TryGo(func() error {
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
					if _, ok := tols[dep]; !ok {
						return gtx.Err()
					}
					tols[d[r].deps[0]] = token{} // propagate tolerance
					depErr = gtx.Err()           // record and continue
				default: // ctx ok
				}
			}
			// no opts short circuit
			if !opts.WithLog && opts.ErrC == nil {
				if err := SafeRun(gtx, d[r].f); err != nil {
					return errors.Join(err, depErr)
				}
				return depErr
			}
			if opts.WithLog {
				defer funcTimer(ctx, "Dep.groupTryGo", opts.Prefix, cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), time.Now())
			}
			if err := SafeRun(gtx, d[r].f); err != nil {
				fName := cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f))
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", fName, err)
				}
				if opts.WithLog {
					slog.ErrorContext(ctx, fmt.Sprintf("[Group Dep.groupTryGo] group %s: %s failed", opts.Prefix, fName), slog.String("err", fmt.Sprintf("%+v", err)))
				}
				return errors.Join(err, depErr)
			}
			return depErr
		})
	}
	return ok
}
