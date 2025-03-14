package example

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/luckycatx/group"
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

	err := group.Go(ctx, nil, c.A)
	assert.Nil(t, err)
	err = group.Go(ctx, nil, c.B, c.C)
	assert.Nil(t, err)
	err = group.Go(ctx, nil, c.D)
	assert.Nil(t, err)

	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())

	//= GroupGo manual signal dependency control
	c = new(exampleCtx)
	s = time.Now()

	type signal = chan struct{}
	var aS, bS, cS, done = make(signal), make(signal), make(signal), make(signal)
	err = group.Go(ctx, nil,
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

	var opts = group.Opts(group.WithDep)
	err := group.Go(ctx, opts,
		group.MakeRunner(c.A).Name(opts, "a"),
		group.MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		group.MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		group.MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Nil(t, err)
	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoDepFF(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = group.Opts(group.WithDep)
	err := group.Go(ctx, opts,
		group.MakeRunner(c.F).Name(opts, "f"), // fast-fail
		group.MakeRunner(c.X).Dep(opts, "f"))

	assert.NotNil(t, err)
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoDepNFF(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = group.Opts(group.WithDep)
	err := group.Go(ctx, opts,
		group.MakeRunner(c.F).Name(opts, "f").Tolerant(opts), // non-fast-fail
		group.MakeRunner(c.X).Dep(opts, "f"))

	assert.NotNil(t, err)
	assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
	assert.Equal(t, 1, c.x)
}

func TestGroupGoTimeout(t *testing.T) {
	t.Parallel()

	var ctx = context.Background()
	c := new(exampleCtx)
	s := time.Now()

	var opts = group.Opts(group.WithTimeout(1 * time.Second))
	err := group.Go(ctx, opts, c.A, c.X)

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

	var opts = group.Opts(group.WithDep, group.WithTimeout(1*time.Second))
	err := group.Go(ctx, opts,
		group.MakeRunner(c.A).Name(opts, "a"),
		group.MakeRunner(c.B).Name(opts, "b").Dep(opts, "a"),
		group.MakeRunner(c.C).Name(opts, "c").Dep(opts, "a"),
		group.MakeRunner(c.D).Name(opts, "d").Dep(opts, "b", "c"))

	assert.Equal(t, "group timeout", err.Error())
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
	// will prevent execution
	assert.Equal(t, 0, c.b)
	assert.Equal(t, 0, c.c)
	assert.Equal(t, 0, c.d)
}

func TestGroupGoVerify(t *testing.T) {
	t.Parallel()

	var opts = group.Opts(group.WithDep)
	var f, x, y, z = func() error { return nil }, func() error { return nil }, func() error { return nil }, func() error { return nil }

	defer func() {
		if x := recover(); x != nil {
			assert.Equal(t, `dependency cycle detected: "c" -> "d" -> "c"`, x)
		}
	}()
	group.MakeRunner(f).Name(opts, "a")
	group.MakeRunner(x).Name(opts, "b").Dep(opts, "a")
	group.MakeRunner(y).Name(opts, "c").Dep(opts, "a", "d")
	group.MakeRunner(z).Name(opts, "d").Dep(opts, "b", "c").Verify(opts)
}
