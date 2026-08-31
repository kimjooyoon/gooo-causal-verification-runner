#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: finalize-ci-artifact.sh CI_OUTPUT_ROOT" >&2
  exit 64
fi

root=$1
summary="$root/cases/ci-summary.json"
process="$root/cases/process/process-guard.json"
compile_ms=$(jq -r '.wall_ms' "$root/compile-observation.json")
build_ms=$(jq -r '.wall_ms' "$root/build-observation.json")
test_ms=$(jq -r '.wall_ms' "$root/test-observation.json")
conformance_ms=$(jq -r '.wall_ms' "$root/conformance-observation.json")
compile_rss=$(jq -r '.peak_rss_kib' "$root/compile-observation.json")
build_rss=$(jq -r '.peak_rss_kib' "$root/build-observation.json")
test_rss=$(jq -r '.peak_rss_kib' "$root/test-observation.json")
conformance_rss=$(jq -r '.peak_rss_kib' "$root/conformance-observation.json")
peak_rss=$(printf '%s\n' "$compile_rss" "$build_rss" "$test_rss" "$conformance_rss" | sort -nr | head -n 1)

inventory=$(jq -c '.inventory' "$root/cases/safe-reuse/verification-receipt.json")
activities=$(jq -r '.activities | length' "$root/cases/safe-reuse/semantic-ir.json")
process_cells=$(jq -r '.cells | length' "$process")
process_cases=$(jq -r '.cases | length' "$process")
executed=$(jq -r '[.cases[].metrics.executed] | add' "$summary")
reused=$(jq -r '[.cases[].metrics.reused] | add' "$summary")
failed=$(jq -r '[.cases[].metrics.failures] | add' "$summary")
unknown=$(jq -r '[.cases[].metrics.unknowns] | add' "$summary")
files=$(find "$root" -type f ! -name 'ci-artifact-summary.json' -printf '%s\n' | wc -l | tr -d ' ')
dirs=$(find "$root" -type d | wc -l | tr -d ' ')
bytes=$(find "$root" -type f ! -name 'ci-artifact-summary.json' -printf '%s\n' | awk '{sum += $1} END {print sum + 0}')

jq -S \
  --argjson inventory "$inventory" \
  --argjson activities "$activities" --argjson process_cells "$process_cells" --argjson process_cases "$process_cases" \
  --argjson compile_ms "$compile_ms" --argjson build_ms "$build_ms" --argjson test_ms "$test_ms" --argjson conformance_ms "$conformance_ms" \
  --argjson compile_rss "$compile_rss" --argjson build_rss "$build_rss" --argjson test_rss "$test_rss" --argjson conformance_rss "$conformance_rss" --argjson peak_rss "$peak_rss" \
  --argjson executed "$executed" --argjson reused "$reused" --argjson failed "$failed" --argjson unknown "$unknown" \
  --argjson files "$files" --argjson dirs "$dirs" --argjson outputs "$files" --argjson bytes "$bytes" \
  --slurpfile process "$process" \
  '. as $root | $root + {
    causal_verifier: {activities:$activities,cases:6,decisions:{CLOSED:2,UNKNOWN:1,REFUTED:3},fixture_metrics:$root.cases},
    process_guard: {cells:$process_cells,cases:$process_cases,decision:$process[0].decision,current_guard_decision:$process[0].current_guard_decision,counts:$process[0].counts,historical_counterexamples:$process[0].historical_counterexamples},
    inventory:$inventory,
    artifacts:{files:$files,dirs:$dirs,outputs:$outputs,bytes:$bytes},
    runtime:{compile_ms:$compile_ms,build_ms:$build_ms,test_ms:$test_ms,conformance_ms:$conformance_ms,peak_rss_kib:$peak_rss,peak_rss_by_operation:{compile:$compile_rss,build:$build_rss,test:$test_rss,conformance:$conformance_rss}},
    tests:{executed:$executed,reused:$reused,failed:$failed,unknown:$unknown},
    direct_main:{bootstrap_direct_main:$process[0].counts.bootstrap_direct_main,historical_post_bootstrap_direct_main:$process[0].counts.historical_post_bootstrap_direct_main,post_guard_direct_main:$process[0].counts.post_guard_direct_main},
    utility_global_core:{state:"UNKNOWN",status:"NOT_MADE"}
  }' "$summary" > "$summary.tmp"
mv "$summary.tmp" "$summary"
cp "$summary" "$root/ci-artifact-summary.json"
