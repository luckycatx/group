package group

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

func Go(ctx context.Context, opts *Options, fs ...func() error) error {
	if len(fs) == 0 {
		return nil
	}

	// no opts short circuit
	if opts == nil {
		g, gtx := errgroup.WithContext(ctx)
		g.SetLimit(len(fs)) // limit defaults to number of funcs
		groupGo(gtx, g, nil, fs...)
		return g.Wait()
	}

	if 0 < opts.Limit && opts.Limit < len(opts.dep) {
		return errors.New("limit cannot be less than the number of funcs with deps")
	}
	if opts.Prefix == "" {
		opts.Prefix = "anonymous"
	}
	if opts.WithLog {
		defer groupTimer(ctx, "Go", opts.Prefix, time.Now())
	}

	g, gtx := errgroup.WithContext(ctx)
	g.SetLimit(cond(opts.Limit > 0, opts.Limit, len(fs))) // limit defaults to number of funcs
	// set timeout for group and fs
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		gtx, cancel = context.WithTimeout(gtx, opts.Timeout)
		defer cancel()
	}
	if opts.dep == nil {
		groupGo(gtx, g, opts, fs...)
	} else {
		// go runners with deps
		// separate ctx for tolerance control
		opts.dep.groupGo(ctx, gtx, g, opts)
		// go runners without deps
		groupGo(gtx, g, opts, filter(fs, func(f func() error) bool { return opts.dep[fptr(f)] == nil })...)
	}

	// outer timeout control
	if opts.Timeout > 0 {
		// only one error is collected by errgroup
		var err, done = error(nil), make(signal)
		go func() {
			defer close(done)
			err = g.Wait()
		}()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-gtx.Done(): // actual timeout
				if errors.Is(gtx.Err(), context.DeadlineExceeded) {
					if opts.WithLog {
						slog.InfoContext(ctx, fmt.Sprintf("[Group Go%s] group %s timeout", cond(opts.dep != nil, " | Dep", ""), opts.Prefix), slog.Duration("after", opts.Timeout))
					}
					return errors.New("group timeout")
				}
				return gtx.Err()
			case <-done:
				return err
			}
		}
	}
	return g.Wait()
}

func TryGo(ctx context.Context, opts *Options, fs ...func() error) (bool, error) {
	if len(fs) == 0 {
		return true, nil
	}

	// no opts short circuit
	if opts == nil {
		g, ctx := errgroup.WithContext(ctx)
		// limit defaults to number of funcs
		g.SetLimit(len(fs))
		return groupTryGo(ctx, g, nil, fs...), g.Wait()
	}

	if 0 < opts.Limit && opts.Limit < len(opts.dep) {
		return false, errors.New("limit cannot be less than the number of funcs with deps")
	}
	if opts.Prefix == "" {
		opts.Prefix = "anonymous"
	}
	if opts.WithLog {
		defer groupTimer(ctx, "TryGo", opts.Prefix, time.Now())
	}

	g, gtx := errgroup.WithContext(ctx)
	g.SetLimit(cond(opts.Limit > 0, opts.Limit, len(fs))) // limit defaults to the number of funcs
	// set timeout for group and fs
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		gtx, cancel = context.WithTimeout(gtx, opts.Timeout)
		defer cancel()
	}
	var ok bool
	if opts.dep == nil {
		ok = groupTryGo(gtx, g, opts, fs...)
	} else {
		// go runners with deps
		// separate ctx for tolerance control
		ok = opts.dep.groupTryGo(ctx, gtx, g, opts)
		// go runners without deps
		ok = ok && groupTryGo(gtx, g, opts, filter(fs, func(r func() error) bool { return opts.dep[fptr(r)] == nil })...)
	}

	// outer timeout control
	if opts.Timeout > 0 {
		// only one error is collected by errgroup
		var err, done = error(nil), make(signal)
		go func() {
			defer close(done)
			err = g.Wait()
		}()
		for {
			select {
			case <-ctx.Done():
				return ok, ctx.Err()
			case <-gtx.Done(): // actual timeout
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					if opts.WithLog {
						slog.InfoContext(ctx, fmt.Sprintf("[Group TryGo%s] group %s timeout", cond(opts.dep != nil, " | Dep", ""), opts.Prefix), slog.Duration("after", opts.Timeout))
					}
					return ok, errors.New("group timeout")
				}
				return ok, ctx.Err()
			case <-done:
				return ok, err
			}
		}
	}
	return ok, g.Wait()
}
