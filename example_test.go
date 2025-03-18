package group

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A -> B -> D
// \        /
// -\> C -/ -> E
// F -> X (F will fail)
// time spent (without E) = A + max(B, C) + D = 4
// time spent (with E) = A + max(B, C) + D + E = 5
// res(d) = (1 + 1) + (1 + 1) = 4
type exampleCtx struct {
	a, b, c, d, e, x int
}

func (c *exampleCtx) A() error {
	time.Sleep(1 * time.Second)
	fmt.Println("A")
	c.a = 1
	return nil
}

func (c *exampleCtx) B() error {
	time.Sleep(1 * time.Second)
	fmt.Println("B")
	c.b = c.a + 1
	return nil
}

func (c *exampleCtx) C() error {
	time.Sleep(2 * time.Second)
	fmt.Println("C")
	c.c = c.a + 1
	return nil
}

func (c *exampleCtx) D() error {
	time.Sleep(1 * time.Second)
	fmt.Println("D")
	c.d = c.b + c.c
	return nil
}

func (c *exampleCtx) E() error {
	time.Sleep(1 * time.Second)
	fmt.Println("E")
	c.e = c.c + 1
	return nil
}

func (c *exampleCtx) F() error {
	time.Sleep(1 * time.Second)
	fmt.Println("F")
	return errors.New("F")
}

func (c *exampleCtx) X() error {
	time.Sleep(1 * time.Second)
	c.x = 1
	return nil
}

func (c *exampleCtx) Res() int {
	return c.d
}

func TestGroupGo(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	//= GroupGo serial dependency control
	c := new(exampleCtx)
	s := time.Now()

	err := Go(ctx, nil, c.A)
	assert.Nil(t, err)
	err = Go(ctx, nil, c.B, c.C)
	assert.Nil(t, err)
	err = Go(ctx, nil, c.D)
	assert.Nil(t, err)

	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())

	//= GroupGo manual signal dependency control
	c = new(exampleCtx)
	s = time.Now()

	type signal = chan struct{}
	var aS, bS, cS, done = make(signal), make(signal), make(signal), make(signal)
	err = Go(ctx, nil,
		func() error {
			defer close(aS)
			return c.A()
		},
		func() error {
			<-aS
			defer close(bS)
			return c.B()
		},
		func() error {
			<-aS
			defer close(cS)
			return c.C()
		},
		func() error {
			_, _ = <-bS, <-cS
			defer close(done)
			return c.D()
		})
	assert.Nil(t, err)

	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoDep(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep)
	err := Go(ctx, opts,
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Nil(t, err)
	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoDepFF(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep)
	err := Go(ctx, opts,
		MakeRunner(c.F).Name(opts, "f"), // fast-fail
		MakeRunner(c.X).Dep(opts, "f"))

	assert.NotNil(t, err)
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoDepNFF(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep)
	err := Go(ctx, opts,
		MakeRunner(c.F).Name(opts, "f").Tolerant(opts), // non-fast-fail
		MakeRunner(c.X).Dep(opts, "f"))

	assert.NotNil(t, err)
	assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
	assert.Equal(t, 1, c.x)
}

//go:norace
func TestGroupGoTimeout(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithTimeout(1 * time.Second))
	err := Go(ctx, opts, c.A, c.X)

	assert.Equal(t, "group timeout", err.Error())
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
	// only the upstream funcs timeout in the dep mode will prevent execution
	time.Sleep(5 * time.Second) // avoid data race
	assert.Equal(t, 1, c.x)
}

func TestGroupGoDepTimeout(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep, WithTimeout(1*time.Second))
	err := Go(ctx, opts,
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Equal(t, "group timeout", err.Error())
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
	// will prevent execution
	assert.Equal(t, 0, c.b)
	assert.Equal(t, 0, c.c)
	assert.Equal(t, 0, c.d)
}

func TestGroupGoVerify(t *testing.T) {
	t.Parallel()

	var opts = Opts(WithDep)
	var f, x, y, z = func() error { return nil }, func() error { return nil }, func() error { return nil }, func() error { return nil }

	defer func() {
		if x := recover(); x != nil {
			// range over map is random
			assert.Contains(t, []string{`dependency cycle detected: "c" -> "d" -> "c"`, `dependency cycle detected: "d" -> "c" -> "d"`}, x)
		}
	}()
	MakeRunner(f).Name(opts, "a")
	MakeRunner(x).Name(opts, "b").Dep(opts, "a")
	MakeRunner(y).Name(opts, "c").Dep(opts, "a", "d")
	MakeRunner(z).Name(opts, "d").Dep(opts, "b", "c").Verify(opts)
}

func TestGroupGoDepWithLog(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep, WithLog)
	err := Go(ctx, opts,
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Nil(t, err)
	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

//= Abnormal Branch

func TestGroupGoDepIllegalLimit(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)

	var opts = Opts(WithDep, WithLimit(3))
	err := Go(ctx, opts,
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Equal(t, "limit cannot be less than the number of funcs with deps", err.Error())
}

func TestGroupGoDepCtxCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// ctx timeout while b & c are running
		time.Sleep(2 * time.Second)
		cancel()
	}()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep)
	err := Go(ctx, opts,
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, c.d, 0)
	assert.Equal(t, float64(3), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoCancelledCtx(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancel
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep)
	err := Go(ctx, opts,
		func() error { return nil },
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"),
		func() error { return nil },
	)

	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, float64(0), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupTryGoNotOk(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = Opts(WithDep, WithLimit(4)) // the funcs without dep will not be executed
	ok, err := TryGo(ctx, opts,
		func() error { return nil },
		MakeRunner(c.A).Name(opts, "a"),
		MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"),
		func() error { return nil },
	)

	assert.False(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}
