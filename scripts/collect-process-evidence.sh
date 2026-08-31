#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: collect-process-evidence.sh REPOSITORY PHASE OUTPUT" >&2
  exit 64
fi

repository=$1
phase=$2
output=$3
work=$(mktemp -d)
mkdir -p "$(dirname "$output")"

gh api "repos/$repository/commits?sha=main&per_page=100" > "$work/main-index.json"
commit_index=0
while IFS= read -r sha; do
  gh api "repos/$repository/commits/$sha/pulls" --jq '[.[].number]' > "$work/prs-$sha.json"
  printf -v ordinal '%03d' "$commit_index"
  jq -S --argjson pull_request_numbers "$(cat "$work/prs-$sha.json")" \
    --arg sha "$sha" \
    '.[] | select(.sha == $sha) | {sha:.sha,parents:[.parents[].sha],pull_request_numbers:$pull_request_numbers}' \
    "$work/main-index.json" > "$work/commit-$ordinal.json"
  commit_index=$((commit_index + 1))
done < <(jq -r '.[].sha' "$work/main-index.json")
jq -S -s '.' "$work"/commit-*.json > "$work/main-commits.json"

for number in 1 2; do
  gh api "repos/$repository/pulls/$number" | jq -S '{number,url:.html_url,state,merged,base_ref:.base.ref,head_ref:.head.ref,head_sha:.head.sha,merge_commit_sha:(.merge_commit_sha // "")}' > "$work/pr-$number.json"
done
jq -S -s '.' "$work"/pr-*.json > "$work/pull-requests.json"

for tag in v0.1.0 v0.1.1; do
  if gh api "repos/$repository/git/ref/tags/$tag" > "$work/ref-$tag.json" 2>/dev/null; then
    object_sha=$(jq -r '.object.sha' "$work/ref-$tag.json")
    object_type=$(jq -r '.object.type' "$work/ref-$tag.json")
    target_sha="$object_sha"
    target_type="$object_type"
    if [ "$object_type" = "tag" ]; then
      gh api "repos/$repository/git/tags/$object_sha" > "$work/tag-$tag.json"
      target_sha=$(jq -r '.object.sha' "$work/tag-$tag.json")
      target_type=$(jq -r '.object.type' "$work/tag-$tag.json")
    fi
    jq -S -n --arg name "$tag" --arg ref_sha "$object_sha" --arg object_sha "$object_sha" \
      --arg object_type "$object_type" --arg target_sha "$target_sha" --arg target_type "$target_type" \
      '{name:$name,ref_sha:$ref_sha,object_sha:$object_sha,object_type:$object_type,target_sha:$target_sha,target_type:$target_type}' > "$work/tag-evidence-$tag.json"
  fi
done
if compgen -G "$work/tag-evidence-*.json" >/dev/null; then
  jq -S -s '.' "$work"/tag-evidence-*.json > "$work/tags.json"
else
  printf '[]\n' > "$work/tags.json"
fi

for tag in v0.1.0 v0.1.1; do
  if gh api "repos/$repository/releases/tags/$tag" > "$work/release-$tag.json" 2>/dev/null; then
    jq -S '{id,tag_name,immutable,draft,prerelease,assets:[.assets[]|{id,name,size,digest}]}' "$work/release-$tag.json" > "$work/release-evidence-$tag.json"
  fi
done
if compgen -G "$work/release-evidence-*.json" >/dev/null; then
  jq -S -s '.' "$work"/release-evidence-*.json > "$work/releases.json"
else
  printf '[]\n' > "$work/releases.json"
fi

main_head=$(jq -r '.[0].sha' "$work/main-index.json")
jq -S -n \
  --arg schema "gooo/causal-verification-runner/github-process-evidence/v1" \
  --arg repository "$repository" --arg phase "$phase" --arg main_head "$main_head" \
  --slurpfile main_commits "$work/main-commits.json" \
  --slurpfile pull_requests "$work/pull-requests.json" \
  --slurpfile tags "$work/tags.json" --slurpfile releases "$work/releases.json" \
  '{schema:$schema,repository:$repository,phase:$phase,main_head:$main_head,main_commits:$main_commits[0],pull_requests:$pull_requests[0],tags:$tags[0],releases:$releases[0]}' \
  > "$output"
