# gooo-causal-verification-runner

An independent, read-only runner for safe verification reuse in Gooo
self-improvement CI. The repository bootstrap is intentionally visible on
`main`; feature development and promotion evidence are recorded separately.

The runner consumes a `.gooo` meta-program, a released semantic graph, exact
test observations, reusable immutable proofs, and an independent full oracle.
It emits a causal plan and machine receipt only under a caller-owned output
directory. It never edits the input checkout or runs target tests.

GitHub Actions is the verification authority. Local tests are not part of the
development contract.

