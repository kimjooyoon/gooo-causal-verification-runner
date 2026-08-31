# Causal verification runner protocol v1

## Decision

The runner makes a test-reuse decision only after it binds a change claim to a
released semantic graph. It traverses known graph edges to derive affected
semantic predicates and required tests. Tests in the affected closure execute
again. Tests outside that closure may be `REUSE` only when an immutable proof
matches every current identity: source, source tree, semantic graph,
toolchain, scenario, test inventory, command, terminal `PASS`, result digest,
and platform-confirmed release immutability.

A cache hit and a historical PASS are inputs to the proof check, never proof
on their own. A self-authored immutable field is subordinate to the external
GitHub release API authority.

## Unknown and refuted resolution

An edge whose status is not `KNOWN` is an unknown impact edge. When it is
reachable from a changed predicate, the runner emits `FULL_VERIFICATION`,
executes every test, uses zero reusable proofs, and preserves:

`stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`.

Known stale identity, non-PASS proof, missing or contradictory immutable
release evidence, cache-only reuse, a hidden counterexample, or a selective
result that differs from the independent full oracle is `REFUTED`. Resolution
precedence is `REFUTED > UNKNOWN > CLOSED`.

## Independent oracle

The full oracle is a separate, digest-bound result set that covers every test.
The selective result map is built from current executed observations and exact
reusable proof result digests. Status and result digest must match the oracle
for every test. Any mismatch is a refutation and names the test IDs. A full
oracle failure is also retained as a refutation.

## Exact metrics

`selected` counts tests requiring current execution. `executed` counts those
with current observations. `reused` counts exact proof-backed tests.
`full_oracle_executed` counts oracle results, `failures` counts oracle `FAIL`
results, and `avoided_executions` is `total_tests - executed`.

Build, test, and conformance milliseconds and before/after wall/RSS values are
integers. Saved time and improvement are integers only when before and after
share identical source, toolchain, and scenario digests; otherwise both are
the literal `UNKNOWN`. No percentage, score, average, or estimate is emitted.

