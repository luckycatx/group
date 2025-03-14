package group

import (
	"fmt"
)

// fptr -> dependency struct
type depMap map[uintptr]*fdep

// dependency struct -> fn, deps
type fdep = struct {
	f    func() error
	deps []string // dependency list, first element is the func itself (name)
}

type signal = chan token
type token = struct{}

type runner func() error

func MakeRunner(f func() error) runner {
	return f
}

func (r runner) Name(opts *Options, name string) runner {
	if opts.dep == nil {
		panic("dep not enabled")
	}
	if opts.dep[fptr(r)] == nil {
		opts.dep[fptr(r)] = &fdep{
			f:    r,
			deps: []string{name}, // empty name is treated as anonymous
		}
	}
	return r
}

func (r runner) Dep(opts *Options, names ...string) runner {
	if opts.dep == nil {
		panic("dep not enabled")
	}
	if len(names) == 0 {
		return r
	}
	if opts.dep[fptr(r)] == nil {
		opts.dep[fptr(r)] = &fdep{
			f: r,
			// auto anonymous for non-named runners
			// anounymous runners can't be dependent
			deps: append(append(make([]string, 0, len(names)+1), ""), filter(names, func(name string) bool { return name != "" })...),
		}
		return r
	}
	// anounymous runners can't be dependent
	opts.dep[fptr(r)].f, opts.dep[fptr(r)].deps = r, append(opts.dep[fptr(r)].deps, filter(names, func(name string) bool { return name != "" })...)
	return r
}

func (r runner) Verify(opts *Options) runner {
	opts.dep.verify(true)
	return r
}

// Marks the runner as non-fast-fail, runners that depend on it will continue to run even if it fails
// error will be collected and wrapped if downstream runners fail
func (r runner) Tolerant(opts *Options) runner {
	if opts.dep == nil {
		panic("dep not enabled")
	}
	// anonymous runner can't be fatal, ignore
	if len(opts.dep[fptr(r)].deps) == 0 || opts.dep[fptr(r)].deps[0] == "" {
		return r
	}
	if opts.tol == nil {
		opts.tol = make(map[string]token, 1)
	}
	opts.tol[opts.dep[fptr(r)].deps[0]] = token{}
	return r
}

func notify(s signal) {
	if s != nil {
		close(s)
	}
}

func (d depMap) verify(panicking bool) string {
	if len(d) == 0 {
		return ""
	}
	graph, src := make(map[string][]string, len(d)), make(map[string]token, len(d))
	for _, fd := range d {
		if len(fd.deps) == 0 {
			continue
		}
		// skip anonymous runner
		if fd.deps[0] == "" {
			continue
		}
		node := fd.deps[0]
		if _, ok := src[node]; ok {
			return fmt.Sprintf("duplicate dependency source %q", node)
		}
		graph[node], src[node] = make([]string, 0, len(fd.deps)-1), token{}
		for i := 1; i < len(fd.deps); i++ {
			if fd.deps[i] != "" {
				graph[node] = append(graph[node], fd.deps[i])
			}
		}
	}

	// check existence
	for _, deps := range graph {
		for _, dep := range deps {
			if _, ok := src[dep]; !ok {
				var missing = fmt.Sprintf("missing dependency %q -> %q", dep, dep)
				if panicking {
					panic(missing)
				}
				return missing
			}
		}
	}

	// check cycle dfs
	stk, visited := make(map[string]token), make(map[string]token)
	var dfs func(node string) string
	dfs = func(node string) string {
		if _, ok := stk[node]; ok {
			return fmt.Sprintf("%q", node)
		}
		if _, ok := visited[node]; ok {
			return ""
		}
		stk[node], visited[node] = token{}, token{}
		for _, dep := range graph[node] {
			if x := dfs(dep); x != "" {
				return fmt.Sprintf("%q -> %s", node, x)
			}
		}
		delete(stk, node)
		return ""
	}
	// check cycle
	for node := range graph {
		if _, ok := visited[node]; !ok {
			if x := dfs(node); x != "" {
				var cycle = fmt.Sprintf("dependency cycle detected: %s", x)
				if panicking {
					panic(cycle)
				}
				return cycle
			}
		}
	}
	return ""
}
