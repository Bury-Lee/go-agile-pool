# Test Harness: Plugin-Based CLI Design & Plugin Authoring Guide

The benchmark/test tool under `test/` is a plugin-based, sub-command style
CLI. This document is the authoritative design reference for the harness
and, more importantly, a **guide for writing new plugins**. For a quick
overview and the scenario scripts see `test/README.md`.

## 1. Why plugins

The legacy tool (archived in `test/old/`) flattened pool options, task
duration models, submit strategies, instrumentation, sampling and profiling
into one `main` with short-flag prefixes (`-T`, `-U`, `-w`, ...). That made
options ambiguous and scenarios hard to compose. The harness now treats
**every capability as a plugin** and the command line as a composition of
plugin segments:

```
agilepool_test [--list | --help [name]] | (--pluginA args... --pluginB args...)
```

The host framework (~a few hundred lines) only splits segments, plans
dependencies and drives the lifecycle. It never knows what a plugin does.
**Adding a capability = one new file + one line in `plugins.go`. The
framework is untouched.**

## 2. Segment syntax

A **segment head** is any token whose text after `--` matches a registered
plugin name. It opens a new segment wherever it appears.

### 2.1 Order contract

Every non-head token belongs to the current segment and reaches the plugin
**verbatim, in command-line order**. The framework never reorders, merges,
deduplicates or groups tokens. Flag-style parsing (`-T fixed --task-base 500`,
flag/value pairs) relies on this; a plugin is free to parse `args` itself.
Order is only lost if the plugin itself chooses to (e.g. `ParseOptions`
turns `key=value` tokens into a map).

- `key=value` — recommended, order-free, self-describing; parsed by helpers.
- bare words — optional, only where a plugin supports positional arguments.
- `-x` / `--unregistered` — allowed pass-through for plugin-owned flag parsing.
- An unregistered `--xxx` before any segment head is a usage error (typo guard).
- A registered name can never be passed as a bare argument (it always starts a
  segment); embed it in a `key=value` value if ever needed.

### 2.2 Parameter helpers

`option.go` provides `ParseOptions`, `Positionals`, `GetString`, `GetInt`,
`GetFloat`, `GetDuration` and `GetBool`. All getters return the given default
when the key is absent and only error when a present value is malformed.

## 3. Plugin contract

Design principle: **minimal core + optional capability interfaces**, detected
by the host through type assertion.

```go
// Required.
type Plugin interface {
    Name() string                                   // unique; used as segment head
    Desc() string                                   // shown by --list / --help
    Run(ctx context.Context, rt *Runtime, args []string) error
}

// Optional: option table for --help <name>. Metadata only; no validation.
type Optioned interface { Options() []Option }      // []Option{Name, Default, Help}

// Optional: session-scoped lifecycle.
type Lifecycle interface {
    Start(ctx context.Context, rt *Runtime, args []string) error
    End(ctx context.Context, rt *Runtime) error
}

// Optional: "must run after these plugins".
type Depender interface { Deps() []string }
```

Why capability interfaces instead of one big interface: a plugin file makes
its own role obvious (pool = lifecycle teardown, submit = dependencies only),
and the host never needs concrete plugin types.

### 3.1 Lifecycle semantics

- `Start` runs before **every** `Run`, in plan order. It receives the segment
  args because envelope plugins (`metrics`, `profile`) must be in place
  (sampler goroutine up, pprof started) before the session begins.
- `Run` runs per segment, in plan order.
- `End` runs after every `Run`, in **reverse** Start order — resources close
  in the opposite order of their creation.
- Both `Start` and `End` must be provided when implementing `Lifecycle`; the
  phase a plugin does not need returns `nil`.
- A failure aborts the session (exit code 1) but already-started lifecycle
  plugins still get `End` called.

### 3.2 Shared data: Store and the performance layering rule

Plugins never reference each other. The only data path is the shared
`Store` (two-level table: plugin name → key → value), provided per session
via `Runtime`:

```go
Provide(rt.Store, "pool", "pool", p)                          // writer: own namespace
p, err := Require[*agilepool.Pool](rt.Store, "pool", "pool")  // reader: dependency namespace
```

Rules:

- Write into **your own namespace** (`plugin.Name()`), immediately after
  parsing. Re-providing the same key overwrites.
- Read only from namespaces of plugins you declared in `Deps()`; dependency
  ordering guarantees they ran first.
- Generics carry the type (`Require[T]`); the type lives at the read site.
- **Performance layering**: the Store only carries *session-level objects*
  (pool handle, configs, factories), touched a handful of times per session.
  Per-task hot data (counters, samples) must stay in the plugin's own
  `atomic` state and must never be written through the Store — that would add
  a lock and a box to every increment. Readers sample the handle, not
  per-tick Store writes.

## 4. Host execution model

```
split argv → plan dependencies → Start (in order) → Run (segment by segment)
            → End (reverse) → exit code
```

1. **Dependency auto-insertion**: when a dependency is not requested, the
   planner inserts it (empty args = defaults) before its dependent, so
   `--submit ...` alone also runs pool+task. Explicitly writing a dependency
   *after* its dependent is an error (exit 2), never a silent reorder: the
   user's segment order is intent. Duplicate segments and dependency cycles
   are errors too.
2. **Start-before-Run** covers envelope needs; **End-in-reverse** tears down
   symmetrically.
3. **Exit codes**: `0` success; `1` runtime error (plugin Start/Run/End);
   `2` usage error (unknown segment, out-of-segment token, misordered
   dependency, duplicate segment, cycle). Errors are printed with a
   `[plugin]` prefix.
4. Ctrl+C cancels the shared `ctx`; plugins watch `ctx.Done()` and End still
   runs.

## 5. Writing a plugin (the guide)

### 5.1 Steps

1. Create `<name>.go` in `test/` (one file per plugin).
2. Implement `Plugin`. Decide which capabilities apply:

   | Capability | Implement when... |
   |---|---|
   | `Options()` | the plugin takes `key=value` options and should show them in `--help <name>` |
   | `Deps()` | it needs outputs of other plugins (pool handle, duration factory) |
   | `Lifecycle` | it must envelope the session (profile), start background work (metrics), or own a resource to close (pool) |

3. Register one line in `plugins.go` (`var allPlugins = []Plugin{...}`).
4. If the plugin produces something for downstream plugins, `Provide` it into
   its own namespace during `Start` (if it must exist before Runs) or `Run`.
5. `--list` / `--help <name>` pick it up automatically.

### 5.2 Template

```go
package main

import (
    "context"
    "fmt"
)

type myPlugin struct{}

func (myPlugin) Name() string { return "my" }
func (myPlugin) Desc() string { return "one-line description for --list" }

func (myPlugin) Options() []Option {
    return []Option{{Name: "x", Default: "1", Help: "what it does"}}
}

func (myPlugin) Run(ctx context.Context, rt *Runtime, args []string) error {
    opts := ParseOptions(args)
    x, err := GetInt(opts, "x", 1)
    if err != nil {
        return err
    }
    // read dependencies:
    // p, err := Require[*agilepool.Pool](rt.Store, "pool", "pool")
    // share outputs:
    // Provide(rt.Store, "my", "result", result)
    _ = x
    return nil
}
```

A complete runnable example pair (provider/consumer, demonstrating every
optional capability) is in `test/example.go`. Register it temporarily in
`plugins.go` to play with it.

### 5.3 Checklist

- [ ] `Name()` unique and stable — it is the CLI surface.
- [ ] Option parsing via the helpers; unknown keys are the plugin's business.
- [ ] Options parsed in `Run` (or in `Start` when the value must be ready
  before other Runs, e.g. metrics interval, profile flags).
- [ ] Errors returned (host prints them with the `[name]` prefix and maps to
  exit code 1); the plugin never calls `os.Exit`.
- [ ] No package-global mutable state; session state lives on the plugin
  struct (register a pointer instance) or is passed via `Runtime`.
- [ ] Hot path stays allocation/atomic-light; Store untouched in loops.
- [ ] One line added to `plugins.go`.

## 6. Existing plugins and legacy mapping

| Plugin | Purpose | Legacy source | Deps |
|---|---|---|---|
| `pool` | create the pool, close on End | old `flag.go` pool flags + `newPool` | — |
| `task` | task duration model (fixed/uniform/normal), provides duration factory | old `flag.go` task flags + `buildDurationFn` | — |
| `submit` | strategies immediate/linear/constant/poisson/phased, blocks until drain | old `runSubmitter` family | `pool`, `task` |
| `hook` | event instrumentation: none / hook counters (trace pending upstream context wiring) | old `hooks.go` `setupHooks` | `pool` |
| `metrics` | periodic runtime/pool/GC/CPU sampling; End: final tick + wait-exit + summary | old `count.go` + main-level shutdown waits | `pool` |
| `profile` | pprof enveloping the session | old `--cpuprofile/--memprofile` wrapping | — |

Legacy flag → new segment mapping used by the scenario scripts
(`run_test.bat`, `run_test.sh`):

| Legacy | New |
|---|---|
| `-T X --task-base N --task-extra E --task-mean M --task-sigma S` | `--task type=X base=N extra=E mean=M sigma=S` |
| `-U S -t N --submit-interval I --submit-jitter J --submit-mean-interval PM --submit-shards SH -P "..."` | `--submit strategy=S num=N interval=I jitter=J mean-interval=PM shards=SH phases="..."` |
| `-w W -c C -m M` | `--pool workers=W container=C mode=M` |
| `-i T -f F -o FILE -e W` | `--metrics interval=T format=F file=FILE wait-exit=W` |
| `--cpuprofile --memprofile` | `--profile cpu=true mem=true` |

## 7. Status and notes

- Metrics CSV/JSON columns and formats are preserved from the legacy tool
  column for column (the CSV header is written on the first tick).
- `--hook mode=trace` is intentionally rejected until the upstream
  context-tracing API (`internal/context`) is wired.
- The old implementation is archived under `test/old/` (not compiled).
