package group

import (
	"errors"
	"log/slog"
	"time"
)

type option func(*Options)

type Options struct {
	Prefix  string        // group name, used for log and metrics, default is "anonymous"
	Limit   int           // concurrency limit
	Timeout time.Duration // group timeout
	ErrC    chan error    // error collector
	WithLog bool

	dep depMap           // dependency map
	tol map[string]token // tolerance map
}

func Opts(opts ...option) *Options {
	opt := &Options{}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

func WithPrefix(s string) option                { return func(o *Options) { o.Prefix = s } }
func WithLimit(x int) option                    { return func(o *Options) { o.Limit = x } }
func WithTimeout(t time.Duration) option        { return func(o *Options) { o.Timeout = t } }
func WithErrorCollector(errC chan error) option { return func(o *Options) { o.ErrC = errC } }
func WithLogger(logger *slog.Logger) option {
	return func(o *Options) { o.WithLog = true; slog.SetDefault(logger) }
}

var (
	WithLog option = func(o *Options) { o.WithLog = true }
	WithDep option = func(o *Options) { o.dep = make(depMap) }
)

func (o *Options) ValidateDep() error {
	if o.dep == nil {
		return nil
	}
	info := o.dep.verify(false)
	return cond(info != "", errors.New(info), nil)
}
