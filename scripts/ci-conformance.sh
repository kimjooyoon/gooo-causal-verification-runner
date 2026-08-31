#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$#" -ne 5 ]; then
  echo "usage: ci-conformance.sh REPOSITORY BINARY OUTPUT BUILD_MS TEST_MS" >&2
  exit 64
fi

repository=$1
binary=$2
output=$3
build_ms=$4
test_ms=$5

before=$(git -C "$repository" status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
mkdir -p "$output"

"$binary" conformance \
  --source "$repository/examples/causal-verification/main.gooo" \
  --contract "$repository/contracts/causal-verification-denominator-v1.json" \
  --corpus "$repository/fixtures/corpus.json" \
  --tree-root "$repository" \
  --output-dir "$output" \
  --build-ms "$build_ms" \
  --test-ms "$test_ms"

jq -e '
  .schema == "gooo/causal-verification-runner/ci-summary/v1" and
  (.cases | length) == 6 and
  ([.cases[] | select(.decision == "CLOSED")] | length) == 2 and
  ([.cases[] | select(.decision == "UNKNOWN")] | length) == 1 and
  ([.cases[] | select(.decision == "REFUTED")] | length) == 3 and
  .authority == {repository_writes:0,local_test_executions:0,cross_project_required_gates:0,verification_authority:"GITHUB_ACTIONS"} and
  .root_readme_excluded == true
' "$output/ci-summary.json" >/dev/null

check_receipt() {
  local case_id=$1
  jq -e '
    .schema == "gooo/causal-verification-runner/receipt/v1" and
    .root_readme_excluded == true and
    .authority.repository_writes == 0 and
    .authority.local_test_executions == 0 and
    .authority.cross_project_required_gates == 0 and
    (.metrics.total_tests|type) == "number" and
    (.metrics.selected|type) == "number" and
    (.metrics.executed|type) == "number" and
    (.metrics.reused|type) == "number" and
    (.metrics.full_oracle_executed|type) == "number" and
    (.metrics.failures|type) == "number" and
    (.metrics.unknowns|type) == "number" and
    (.metrics.before_wall_ms|type) == "number" and
    (.metrics.after_wall_ms|type) == "number" and
    (.metrics.before_peak_rss_kib|type) == "number" and
    (.metrics.after_peak_rss_kib|type) == "number" and
    (.metrics.build_ms|type) == "number" and
    (.metrics.test_ms|type) == "number" and
    (.metrics.conformance_ms|type) == "number" and
    (.metrics.avoided_executions|type) == "number" and
    ([.. | objects | keys[]? | select(test("percent|percentage|score|average"; "i"))] | length) == 0
  ' "$output/$case_id/verification-receipt.json" >/dev/null
}

for case_id in safe-reuse transitive-impact unknown-edge-full-fallback stale-proof-rejection hidden-counterexample cache-hit-only; do
  check_receipt "$case_id"
done

jq -e '
  .decision == "CLOSED" and .selection_mode == "CAUSAL_SELECT" and
  .metrics.total_tests == 2 and .metrics.selected == 1 and .metrics.executed == 1 and .metrics.reused == 1 and
  .metrics.full_oracle_executed == 2 and .metrics.failures == 0 and .metrics.unknowns == 0 and
  .metrics.before_wall_ms == 100 and .metrics.after_wall_ms == 70 and
  .metrics.before_peak_rss_kib == 200 and .metrics.after_peak_rss_kib == 180 and .metrics.avoided_executions == 1 and
  .full_oracle_comparison.state == "CLOSED" and
  ([.tests[] | select(.action == "REUSE")] | length) == 1
' "$output/safe-reuse/verification-receipt.json" >/dev/null

jq -e '
  .decision == "CLOSED" and .selection_mode == "CAUSAL_SELECT" and
  .required_tests == ["test_leaf","test_root"] and
  .metrics.total_tests == 3 and .metrics.selected == 2 and .metrics.executed == 2 and .metrics.reused == 1 and
  ([.invalidation_frontier[] | select(.edge_id == "edge-middle-leaf")] | length) == 1
' "$output/transitive-impact/verification-receipt.json" >/dev/null

jq -e '
  .decision == "UNKNOWN" and .selection_mode == "FULL_VERIFICATION" and
  .metrics.selected == 2 and .metrics.executed == 2 and .metrics.reused == 0 and .metrics.avoided_executions == 0 and
  ([.unknowns[] | select(.stage == "AFFECTED_SEMANTIC_PREDICATES" and .step == "resolve-impact-edge" and .reason == "UNKNOWN_IMPACT_EDGE" and .unknown_class == "CAUSAL_EDGE_UNKNOWN" and .next_operation == "OBTAIN_SEMANTIC_GRAPH_EDGE_PROOF" and (.blocked_by == ["edge:edge-component-secret-unknown"]))] | length) == 1 and
  ([.tests[] | select(.action == "REUSE")] | length) == 0
' "$output/unknown-edge-full-fallback/verification-receipt.json" >/dev/null

jq -e '.decision == "REFUTED" and any(.refutations[]; startswith("STALE_PROOF_REJECTED_SCENARIO_DIGEST")) and .metrics.reused == 0 and .metrics.executed == 2' "$output/stale-proof-rejection/verification-receipt.json" >/dev/null
jq -e '.decision == "REFUTED" and .full_oracle_comparison.reason == "SELECTIVE_FULL_ORACLE_MISMATCH" and any(.refutations[]; startswith("HIDDEN_COUNTEREXAMPLE_REFUTED")) and .metrics.failures == 1' "$output/hidden-counterexample/verification-receipt.json" >/dev/null
jq -e '.decision == "REFUTED" and any(.refutations[]; . == "CACHE_HIT_WITHOUT_EXACT_PROOF") and .metrics.reused == 0' "$output/cache-hit-only/verification-receipt.json" >/dev/null

after=$(git -C "$repository" status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
test "$before" = "$after"
test "$(find "$output" -type f | wc -l | tr -d ' ')" -ge 37

