# gooo-causal-verification-runner

An independent, read-only runner for safe verification reuse in Gooo
self-improvement CI.

The `.gooo` meta-program is the authority for the six-stage chain:

`change claim → affected semantic predicates → required tests → reusable proofs → executed tests → independent full-oracle comparison`

The runner consumes a released semantic graph, exact current observations,
immutable proof receipts, and an independent full oracle. It emits a causal
plan, machine receipt, and human report only under a caller-owned output
directory. It never edits the input checkout or runs target tests.

Known impact is traversed transitively. An unknown edge in the affected region
lowers resolution to `FULL_VERIFICATION` and preserves the six-field UNKNOWN
frontier. A stale proof, cache-only reuse, platform immutability contradiction,
hidden counterexample, or selective/full-oracle mismatch is `REFUTED`.

Every report uses exact integer counts and milliseconds: total tests, selected,
executed, reused, full-oracle executed, failures, unknowns, before/after wall
time and peak RSS, build/test/conformance time, and avoided executions. Saved
time and improvement are `UNKNOWN` unless the before/after pair has identical
source, toolchain, and scenario digests. No score or percentage is emitted.

GitHub Actions is the verification authority. Local tests are not part of the
development contract. The root README is excluded from the physical inventory
and tree digest.

## CI-only usage

The workflow installs Go 1.27, builds and vets the runner, runs the Go test
suite in Actions, and evaluates every fixture under `$RUNNER_TEMP`. The
conformance script asserts the five required scenarios plus the cache-only
guard:

- `safe-reuse`
- `transitive-impact`
- `unknown-edge-full-fallback`
- `stale-proof-rejection`
- `hidden-counterexample`
- `cache-hit-only`

The release workflow prepares `v0.1.0` archive, checksum, and manifest assets
under runner-owned temporary storage. It does not create or mutate a GitHub
release; publication and the immutable platform audit remain human-controlled.
