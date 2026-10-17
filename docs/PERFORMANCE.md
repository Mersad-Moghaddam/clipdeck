# Performance evidence — v0.4 work in progress

Baseline backend: `a3e7cbd55f3f2c5374c511388a3dd873660e5df8`.
Plugin baseline: `6ce5190641f26c72dab6159bb87856b6b4737ddf`.
Host: Intel Core i5-6500 @ 3.20 GHz, linux/amd64, Go 1.27.2,
Hyprland session. Tests and vet passed; baseline race suite passed.

Command: `go test -run '^$' -bench BenchmarkSearch10000Items -benchmem -count=5 ./internal/store`.
Existing synthetic 10,000-row benchmark; setup excluded from timing.

| Run | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| 1 | 33342050 | 49728 | 673 |
| 2 | 32999113 | 49683 | 673 |
| 3 | 36117342 | 49687 | 673 |
| 4 | 44839009 | 51149 | 691 |
| 5 | 32538163 | 49286 | 667 |

This aggregate cycles queries; it is not a per-query percentile. One baseline
query with a type filter has a correctness bug, so it must not be used to claim
a speedup from fixing missing results. Matched post-change measurements,
query distributions, RSS, frame traces and UI latency remain pending.
No performance target is claimed achieved.

## Filtered query index experiment

`BenchmarkQueryClasses` seeds 10,000 synthetic text clips; each sub-benchmark
times only QueryItems calls. Same machine/toolchain, one run before/after:

| Query | Before ns/op | Stable recent index ns/op |
|---|---:|---:|
| Synthetic | 41738980 | 40714213 |
| item 9999 | 1167236 | 1153057 |
| no-such-clip | 151360 | 150596 |
| it | 21713410 | 368050 |
| type:text unique words | 51661672 | 50523818 |

An EXPLAIN regression reproduced an avoidable temporary sort for the new
stable recent order. An index matching `pinned DESC, updated_at DESC, id DESC`
removes it, substantially improving short-query first-page retrieval in this
fixture. These single-run means are not p95 or a validated cross-workload
release performance result. Common-match FTS ranking remains slow (~40–50 ms).
The ≥30% common-query improvement gate remains unmet; publication is blocked.
Rejected experiment: a materialized narrow ranked-key CTE before fetching clip
payloads increased common-match means to ~43.7/53.7 ms and no-match to ~0.216 ms;
removed rather than retain an unproven optimization.
CPU profile includes fixture seeding; its whole-process percentages must not
be presented as search-only percentages.
