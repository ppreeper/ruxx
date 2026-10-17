# Performance

ruxx is built to be fast enough to run on every save. This page records what
"fast" means in practice, how the numbers were produced, and where the time
goes.

All measurements below are from one machine (AMD Ryzen 7 7730U, 8 cores / 16
threads, Linux), Go 1.27.1, on a quiet system. Reproduce them with
[`scripts/bench.sh`](../scripts/bench.sh).

## Micro-benchmarks

`go test -run '^$' -bench . -benchtime=300ms ./internal/...`

| Benchmark | Input | ns/op | Throughput | Allocs/op |
|---|---|---|---|---|
| `syntax/BenchmarkParse` | 31 KB Odoo view | 207 µs | 151 MB/s | 3,768 |
| `syntax/BenchmarkParseLarge` | 628 KB Odoo view | 5.26 ms | 119 MB/s | 75,023 |
| `syntax/BenchmarkUnparse` | 31 KB tree | 21 µs | 1,495 MB/s | 1 |
| `syntax/BenchmarkRoundTrip` | parse + unparse | 236 µs | 133 MB/s | 3,769 |
| `rules/BenchmarkCheck` | `select = ["ALL"]`, 50 records | 789 µs | — | 3,987 |
| `rules/BenchmarkCheckDefault` | default set, 50 records | 217 µs | — | 441 |
| `formatter/BenchmarkFormat` | one-line file, 50 records | 438 µs | 39 MB/s | 5,890 |
| `formatter/BenchmarkFormatIdempotent` | already formatted | 557 µs | 61 MB/s | 7,046 |
| `formatter/BenchmarkFormatAttributeSort` | `attribute-sort = "alphabetical"` | 516 µs | 33 MB/s | 7,440 |
| `index/BenchmarkBuild/cold` | 12 modules × 25 files | 2.58 ms | — | 19,998 |
| `index/BenchmarkBuild/cold-single-worker` | same, `Workers: 1` | 4.32 ms | — | 19,904 |
| `index/BenchmarkBuild/cached` | same, cache reused | 1.38 ms | — | 5,818 |
| `cli/BenchmarkCheckFiles` | 50 files, end to end, `-n` | 0.89 ms | — | 4,554 |
| `cli/BenchmarkCheckFilesCached` | 50 files, warm cache | 0.89 ms | — | 3,359 |
| `cli/BenchmarkFormatCheckFiles` | 50 files, `format --check` | 0.85 ms | — | 3,580 |

Notes:

- Parsing dominates a lint run. `rules/BenchmarkCheck` with the default set is
  ~3.6× cheaper than `select = ["ALL"]`, which is why the default set is
  `correctness` only.
- `Unparse` allocates once, no matter the document size — the parser keeps the
  source bytes and re-emits untouched spans from them.
- The formatter costs roughly the same as parsing plus one lint pass, and stays
  in that range when attribute sorting or the idempotent (already formatted)
  path is taken.
- The index build parallelises across modules: the single-worker run is ~1.7×
  slower on a 12-module project, and the cached run skips parsing entirely
  (only `stat` calls plus JSON decoding).

## End-to-end: 45,356 files

The corpus is five Odoo releases — 15.0, 17.0, 18.0, 19.0, 20.0 — each with
`odoo/addons` and `enterprise` walked recursively (45,356 XML files, 437 MB).

```console
$ time ruxx check -n <45k files>
real    0m7.7s   user    0m10.4s   sys    5.9s
```

That is ~5,900 files/second, with the ~1,006 findings (all verified true
positives) unchanged. Memory peaks around 550–600 MB for the whole sweep.

`-n` is used so the numbers reflect parsing and linting rather than cache
lookups; without it a second run is faster still because each file's
diagnostics come from `.ruxx_cache/`.

## Cross-file index cache

When `[odoo].modules-path` is configured, ruxx builds an index of every
module (manifests, xmlid definitions, `res.groups`, `ir.model.access.csv`).
The index is persisted to `<cache-dir>/index.json` and reused while a module's
file set is unchanged — invalidation is by `size:mtime` stamp per file.

Measured on 20.0 `odoo/addons` + `enterprise` (12,143 files, 9.5 MB
`index.json`):

| Run | Command | Wall |
|---|---|---|
| cold, no cache | `ruxx check -n --select O018 …` | 0.33 s |
| cold, writing cache | `ruxx check --select O018 --cache-dir .cache …` | 0.44 s |
| warm, cache reused | same, second run | 0.21 s |

The warm run is ~2× faster than cold, and touch-based invalidation means
editing one file re-parses only that module.

Two behaviours worth knowing:

- `--no-cache` / `-n` disables the index cache as well as the diagnostics
  cache, so benchmark runs always measure real work.
- `ruxx clean` removes `index.json` along with the diagnostics entries.

## The 16× index bug (fixed)

Early index caching had a data race that made the indexed path *slower* than
no index at all: each of the 16 file workers raced past the unsynchronised
memo map and built the entire index itself. `strace` showed every XML file and
directory opened 17 times (16 workers + 1 lint pass) instead of twice, and the
config file opened once per linted file.

| Scenario | Before | After |
|---|---|---|
| 45k sweep, index configured, `-n` | 29.8 s / 4.3 GB RSS | 9.5 s / 604 MB |
| 45k sweep, index + cold cache | 34.7 s | 10.4 s |
| 45k sweep, index + warm cache | 19.0 s | 8.2 s |
| 45k sweep, no index (baseline) | 8.6 s | 8.6 s |
| `openat` calls, 4,987 files | 230,823 | 42,662 |

The fix moved all per-run shared state (resolved config, diagnostics cache,
cross-file index) into a single `runState` with mutex-guarded memos, so N
workers share one index build and config discovery is memoised per directory.
An indexed warm-cache run is now faster than the no-index baseline.
