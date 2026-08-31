# Reuse boundary investigation

This runner is an external consumer boundary. It was designed after a
read-only review of the public repositories below:

- [gooo-verification-reuse](https://github.com/kimjooyoon/gooo-verification-reuse)
  owns whole-run identity binding and fail-closed reuse authorization. This
  runner consumes the same evidence discipline but owns test-level causal
  planning and full-oracle comparison.
- [gooo-test-frontier](https://github.com/kimjooyoon/gooo-test-frontier)
  owns semantic test-frontier traversal, exact per-test status accounting, and
  external release immutability precedence. This runner reuses those boundary
  rules while adding the execution fallback and independent oracle result
  comparison required by the causal runner.
- [meta-ontology-go](https://github.com/kimjooyoon/meta-ontology-go) owns the
  `.gooo` surface and its semantic compiler boundary. The runner's source
  reader binds only the declared activity inventory; the affected graph is an
  input from the released compiler output and is not reconstructed from file
  globs.

The runner therefore does not claim that a cache key is evidence, does not
invent a zero-duration reuse observation, does not treat self-asserted release
immutability as external authority, and does not suppress target CI. Its
caller-owned outputs contain the causal plan and receipts needed by a later
CI-controlled integration.

## Authority boundary

The runner reads source, contract, semantic graph, proof receipts, current
test observations, full-oracle results, and exact timing pairs. It writes only
the supplied empty output directory. `repository_writes`,
`local_test_executions`, and `cross_project_required_gates` are all exact zero
in its receipts. The only verification authority named by the protocol is
`GITHUB_ACTIONS`.

