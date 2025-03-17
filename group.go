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

			var err error
			if opts.WithLog {
				defer funcTimer(ctx, "groupGo", opts.Prefix, funcName(f), time.Now(), err)
			}

			if err = SafeRun(ctx, f); err != nil {
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", funcName(f), err)
				}
			}
			return err
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

			var err error
			if opts.WithLog {
				defer funcTimer(ctx, "groupTryGo", opts.Prefix, funcName(f), time.Now(), err)
			}

			if err = SafeRun(ctx, f); err != nil {
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", funcName(f), err)
				}
			}
			return err
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

			var err error // record dep err
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
					opts.tol[d[r].deps[0]], err = token{}, gtx.Err()
				default: // ctx ok
				}
			}

			// no opts short circuit
			if !opts.WithLog && opts.ErrC == nil {
				if ferr := SafeRun(gtx, d[r].f); ferr != nil {
					return errors.Join(ferr, err)
				}
				return err
			}

			var ferr error // actual err
			if opts.WithLog {
				defer funcTimer(ctx, "Dep.groupGo", opts.Prefix, cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), time.Now(), ferr)
			}

			if ferr = SafeRun(gtx, d[r].f); ferr != nil {
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), ferr)
				}
				return errors.Join(ferr, err)
			}
			return err
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

			var err error // record dep err
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
					opts.tol[d[r].deps[0]], err = token{}, gtx.Err()
				default: // ctx ok
				}
			}

			// no opts short circuit
			if !opts.WithLog && opts.ErrC == nil {
				if ferr := SafeRun(gtx, d[r].f); ferr != nil {
					return errors.Join(ferr, err)
				}
				return err
			}

			var ferr error
			if opts.WithLog {
				defer funcTimer(ctx, "Dep.groupTryGo", opts.Prefix, cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), time.Now(), ferr)
			}

			if ferr = SafeRun(gtx, d[r].f); ferr != nil {
				if opts.ErrC != nil {
					opts.ErrC <- fmt.Errorf("%s failed: %w", cond(d[r].deps[0] != "", d[r].deps[0], funcName(d[r].f)), ferr)
				}
				return errors.Join(ferr, err)
			}
			return err
		})
	}
	return ok
}
