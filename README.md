# [Group] Concurrency Kit

## Built on top of std errgroup
## Much less overhead by optional features selected

## APIs
`group.Go(), group.TryGo()`

---

`group.Opts(group.With...)`

`Options.VerifyDep`

---

`group.MakeRunner`

`runner.Name, runner.Dep, runner.Verify`

---

## Options
Get options by `group.Opts(group.With...)`

or by `group.Options{...}`

## Dependencies
Get dependency attached options: `var opts = group.Opts(group.WithDep)` (dependencies cannot be assigned directly)

Then use `Go` with `MakeRunner` and set the dependencies by assigning name (`runner.Name`) and dependencies (`runner.Dep`) string to the runner

*(You have to set the dependencies correctly, there's no guarantee if you set them wrong)*

***! Note: Multiple MakeRunners for the same function are still considered as one instance***

***! This can cause undefined behavior, avoid it unless you really know what you're doing***

## Usage
Refer to the example package in this repo

## Verify
Dependencies can be verified by using `Options.Veirfy` and `runner.Verify` (dependencies existance and cycle check)

`Options.Verify` will return an error if the dependency is broken

`runner.Verify` can be called in the invocation chain, which will check for the dependencies set prior to the call and **panic** if the dependency is broken

## Benchmark
```
goos: darwin
goarch: arm64
pkg: github.com/luckycatx/group/benchmark
cpu: Apple M3 Pro
BenchmarkGo

BenchmarkGo/TinyWorkload
BenchmarkGo/TinyWorkload/StdGoroutine
BenchmarkGo/TinyWorkload/StdGoroutine-12         	  468334	      2189 ns/op	     256 B/op	      11 allocs/op
BenchmarkGo/TinyWorkload/StdErrGroup
BenchmarkGo/TinyWorkload/StdErrGroup-12          	  466860	      2428 ns/op	     400 B/op	      13 allocs/op
BenchmarkGo/TinyWorkload/GroupGo
BenchmarkGo/TinyWorkload/GroupGo-12              	  357048	      3547 ns/op	    1104 B/op	      25 allocs/op
BenchmarkGo/TinyWorkload/GroupGoWithOpts
BenchmarkGo/TinyWorkload/GroupGoWithOpts-12      	   91026	     13351 ns/op	    4099 B/op	      89 allocs/op

BenchmarkGo/SmallWorkload
BenchmarkGo/SmallWorkload/StdGoroutine
BenchmarkGo/SmallWorkload/StdGoroutine-12        	    1029	   1160579 ns/op	   12049 B/op	     201 allocs/op
BenchmarkGo/SmallWorkload/StdErrGroup
BenchmarkGo/SmallWorkload/StdErrGroup-12         	    1021	   1161205 ns/op	   12160 B/op	     203 allocs/op
BenchmarkGo/SmallWorkload/GroupGo
BenchmarkGo/SmallWorkload/GroupGo-12             	    1035	   1153731 ns/op	   17201 B/op	     305 allocs/op
BenchmarkGo/SmallWorkload/GroupGoWithOpts
BenchmarkGo/SmallWorkload/GroupGoWithOpts-12     	    1017	   1184682 ns/op	   39022 B/op	     819 allocs/op

BenchmarkGo/MediumWorkload
BenchmarkGo/MediumWorkload/StdGoroutine
BenchmarkGo/MediumWorkload/StdGoroutine-12       	     207	   5809449 ns/op	  120301 B/op	    2003 allocs/op
BenchmarkGo/MediumWorkload/StdErrGroup
BenchmarkGo/MediumWorkload/StdErrGroup-12        	     206	   5817464 ns/op	  120482 B/op	    2004 allocs/op
BenchmarkGo/MediumWorkload/GroupGo
BenchmarkGo/MediumWorkload/GroupGo-12            	     204	   5802622 ns/op	  168550 B/op	    3006 allocs/op
BenchmarkGo/MediumWorkload/GroupGoWithOpts
BenchmarkGo/MediumWorkload/GroupGoWithOpts-12    	     195	   6035044 ns/op	  378497 B/op	    8028 allocs/op

BenchmarkGo/LargeWorkload
BenchmarkGo/LargeWorkload/StdGoroutine
BenchmarkGo/LargeWorkload/StdGoroutine-12        	      25	  49994632 ns/op	 1209252 B/op	   20063 allocs/op
BenchmarkGo/LargeWorkload/StdErrGroup
BenchmarkGo/LargeWorkload/StdErrGroup-12         	      25	  43359888 ns/op	 1204033 B/op	   20027 allocs/op
BenchmarkGo/LargeWorkload/GroupGo
BenchmarkGo/LargeWorkload/GroupGo-12             	      26	  46937872 ns/op	 1701123 B/op	   30185 allocs/op
BenchmarkGo/LargeWorkload/GroupGoWithOpts
BenchmarkGo/LargeWorkload/GroupGoWithOpts-12     	      24	  45862292 ns/op	 3810502 B/op	   80464 allocs/op
```

```
goos: darwin
goarch: arm64
pkg: github.com/luckycatx/group/benchmark
cpu: Apple M3 Pro
BenchmarkGoDep

BenchmarkGoDep/StdErrGroup
BenchmarkGoDep/StdErrGroup-12         	  675750	      1675 ns/op	     640 B/op	      17 allocs/op
BenchmarkGoDep/GroupGo
BenchmarkGoDep/GroupGo-12             	  211592	      6260 ns/op	    2272 B/op	      43 allocs/op
```