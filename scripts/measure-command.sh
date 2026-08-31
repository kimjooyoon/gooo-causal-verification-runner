#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$#" -lt 4 ]; then
  echo "usage: measure-command.sh METRICS RESULT LOG OPERATION_ID COMMAND [ARGS...]" >&2
  exit 64
fi

metrics=$1
result_path=$2
log_path=$3
operation_id=$4
shift 4

time_output="${metrics}.time"
mkdir -p "$(dirname "$metrics")" "$(dirname "$result_path")" "$(dirname "$log_path")"
status=0
if LC_ALL=C /usr/bin/time -f '%e %M' -o "$time_output" "$@" >"$log_path" 2>&1; then
  status=0
else
  status=$?
fi
read -r seconds rss < "$time_output"
wall_ms=$(awk -v seconds="$seconds" 'BEGIN { printf "%d", (seconds * 1000) + 0.5 }')
peak_rss_kib=$(awk -v rss="$rss" 'BEGIN { printf "%d", rss + 0 }')
terminal_result="PASS"
if [ "$status" -ne 0 ]; then
  terminal_result="FAIL"
fi

jq -S -n \
  --arg operation_id "$operation_id" \
  --arg status "$([ "$status" -eq 0 ] && echo PASS || echo FAIL)" \
  --arg terminal_result "$terminal_result" \
  --argjson wall_ms "$wall_ms" \
  --argjson peak_rss_kib "$peak_rss_kib" \
  '{schema:"gooo/causal-verification-runner/command-observation/v1",operation_id:$operation_id,status:$status,terminal_result:$terminal_result,wall_ms:$wall_ms,peak_rss_kib:$peak_rss_kib}' \
  > "$metrics"

if [ "$status" -ne 0 ]; then
  cat "$log_path" >&2
  exit "$status"
fi

