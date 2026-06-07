#!/usr/bin/env bash
set -u

search_query=""
include_search_all=0
max_all_messages=25
out_dir=""
preview_bytes=4000
inline_raw=0

usage() {
  cat <<'USAGE'
Usage:
  scripts/agent-smoke-transcript.sh [--query TEXT] [--include-search-all] [--max-all-messages N] [--preview-bytes N] [--inline-raw] [--out-dir DIR]

Runs the real imsgcrawl binary on PATH and captures exact stdout/stderr for a
progressive agent smoke pass. Exact raw outputs are written only to the local
output directory, which defaults to /tmp. The default review file contains
bounded previews plus raw file paths, not a full giant transcript.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --query|--search-query)
      if [[ $# -lt 2 ]]; then
        echo "--query requires a value" >&2
        exit 2
      fi
      search_query=$2
      shift 2
      ;;
    --include-search-all)
      include_search_all=1
      shift
      ;;
    --max-all-messages)
      if [[ $# -lt 2 ]]; then
        echo "--max-all-messages requires a value" >&2
        exit 2
      fi
      max_all_messages=$2
      shift 2
      ;;
    --preview-bytes)
      if [[ $# -lt 2 ]]; then
        echo "--preview-bytes requires a value" >&2
        exit 2
      fi
      preview_bytes=$2
      shift 2
      ;;
    --inline-raw)
      inline_raw=1
      shift
      ;;
    --out-dir)
      if [[ $# -lt 2 ]]; then
        echo "--out-dir requires a value" >&2
        exit 2
      fi
      out_dir=$2
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v imsgcrawl >/dev/null 2>&1; then
  echo "imsgcrawl not found on PATH; run go install ./cmd/imsgcrawl first" >&2
  exit 127
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq not found on PATH" >&2
  exit 127
fi

if [[ -z "$out_dir" ]]; then
  out_dir="${TMPDIR:-/tmp}/imsgcrawl-agent-smoke-$(date -u +%Y%m%dT%H%M%SZ)-$$"
fi
raw_dir="$out_dir/raw"
mkdir -p "$raw_dir"

review="$out_dir/review.txt"
transcript="$out_dir/transcript.txt"
archive="$out_dir/archive.db"
commands_tsv="$out_dir/commands.tsv"
manifest_jsonl="$out_dir/manifest.jsonl"
failures=0
step=0
last_stdout=""

quote_command() {
  printf '%q ' "$@"
}

append_preview() {
  local label=$1
  local path=$2
  local bytes
  bytes=$(wc -c <"$path" | tr -d ' ')
  {
    printf '\n----- %s PREVIEW BEGIN -----\n' "$label"
    printf 'file: %s\nbytes: %s\n' "$path" "$bytes"
    if [[ "$bytes" -eq 0 ]]; then
      printf '(empty)\n'
    elif [[ "$bytes" -le "$preview_bytes" ]]; then
      cat "$path"
      printf '\n'
    else
      local head_bytes=$((preview_bytes / 2))
      local tail_bytes=$((preview_bytes - head_bytes))
      printf 'preview_truncated: true\n'
      printf 'Open the raw file above for exact full output.\n\n'
      head -c "$head_bytes" "$path"
      printf '\n... PREVIEW TRUNCATED: %s bytes total ...\n' "$bytes"
      tail -c "$tail_bytes" "$path"
      printf '\n'
    fi
    printf -- '----- %s PREVIEW END -----\n' "$label"
  } >>"$review"
}

run_step() {
  local name=$1
  shift
  step=$((step + 1))
  local id
  id=$(printf '%03d' "$step")
  local slug
  slug=$(printf '%s' "$name" | tr -cs '[:alnum:]' '-' | sed 's/^-//; s/-$//' | tr '[:upper:]' '[:lower:]')
  local stdout_path="$raw_dir/$id-$slug.stdout"
  local stderr_path="$raw_dir/$id-$slug.stderr"

  "$@" >"$stdout_path" 2>"$stderr_path"
  local code=$?
  last_stdout="$stdout_path"
  local stdout_bytes
  local stderr_bytes
  stdout_bytes=$(wc -c <"$stdout_path" | tr -d ' ')
  stderr_bytes=$(wc -c <"$stderr_path" | tr -d ' ')

  {
    printf '\n================================================================================\n'
    printf '[%s] %s\n' "$id" "$name"
    printf '================================================================================\n'
    printf 'command: '
    quote_command "$@"
    printf '\n'
    printf 'exit_code: %s\n' "$code"
    printf 'stdout_file: %s\n' "$stdout_path"
    printf 'stderr_file: %s\n' "$stderr_path"
    printf 'stdout_bytes: %s\n' "$stdout_bytes"
    printf 'stderr_bytes: %s\n' "$stderr_bytes"
  } >>"$review"

  append_preview "STDOUT $id" "$stdout_path"
  append_preview "STDERR $id" "$stderr_path"

  if [[ "$inline_raw" -eq 1 ]]; then
    {
      printf '\n================================================================================\n'
      printf '[%s] %s\n' "$id" "$name"
      printf '================================================================================\n'
      printf 'command: '
      quote_command "$@"
      printf '\n'
      printf 'exit_code: %s\n' "$code"
      printf 'stdout_file: %s\n' "$stdout_path"
      printf 'stderr_file: %s\n' "$stderr_path"
      printf 'stdout_bytes: %s\n' "$stdout_bytes"
      printf 'stderr_bytes: %s\n' "$stderr_bytes"
      printf '\n----- STDOUT BEGIN %s -----\n' "$id"
      cat "$stdout_path"
      printf '\n----- STDOUT END %s -----\n' "$id"
      printf '\n----- STDERR BEGIN %s -----\n' "$id"
      cat "$stderr_path"
      printf '\n----- STDERR END %s -----\n' "$id"
    } >>"$transcript"
  fi

  printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$name" "$code" "$stdout_path" "$stderr_path" >>"$commands_tsv"
  jq -nc \
    --arg id "$id" \
    --arg name "$name" \
    --arg command "$(quote_command "$@")" \
    --argjson exit_code "$code" \
    --arg stdout_file "$stdout_path" \
    --arg stderr_file "$stderr_path" \
    --argjson stdout_bytes "$stdout_bytes" \
    --argjson stderr_bytes "$stderr_bytes" \
    '{id:$id,name:$name,command:$command,exit_code:$exit_code,stdout_file:$stdout_file,stderr_file:$stderr_file,stdout_bytes:$stdout_bytes,stderr_bytes:$stderr_bytes}' >>"$manifest_jsonl"

  if [[ "$code" -ne 0 ]]; then
    failures=$((failures + 1))
  fi
}

append_note() {
  {
    printf '\n================================================================================\n'
    printf 'NOTE\n'
    printf '================================================================================\n'
    printf '%s\n' "$1"
  } >>"$review"
  if [[ "$inline_raw" -eq 1 ]]; then
    {
      printf '\n================================================================================\n'
      printf 'NOTE\n'
      printf '================================================================================\n'
      printf '%s\n' "$1"
    } >>"$transcript"
  fi
}

cat >"$review" <<EOF
imsgcrawl Agent Smoke Review

Generated at: $(date -u +%Y-%m-%dT%H:%M:%SZ)
Binary: $(command -v imsgcrawl)
Output directory: $out_dir
Raw output directory: $raw_dir
Temporary archive: $archive
Preview bytes per stream: $preview_bytes

This review contains bounded previews. Exact raw stdout/stderr for each command
are in the raw output directory and listed in manifest.jsonl/commands.tsv. Treat
all artifacts as private local crawler data. Do not commit them, paste them into
public systems, or send them off-machine without explicit user consent.
EOF

if [[ "$inline_raw" -eq 1 ]]; then
  cat >"$transcript" <<EOF
imsgcrawl Agent Smoke Transcript

Generated at: $(date -u +%Y-%m-%dT%H:%M:%SZ)
Binary: $(command -v imsgcrawl)
Output directory: $out_dir
Temporary archive: $archive

This transcript intentionally contains exact local command output inline. Treat
it as private local crawler data. Do not commit it, paste it into public systems,
or send it off-machine without explicit user consent.
EOF
fi

: >"$commands_tsv"
: >"$manifest_jsonl"

run_step "version" imsgcrawl --version
run_step "top-help" imsgcrawl --help
run_step "help-chats-topic" imsgcrawl help chats
run_step "help-chats-flag" imsgcrawl chats --help
run_step "help-messages-topic" imsgcrawl help messages
run_step "help-search-topic" imsgcrawl help search
run_step "help-contacts-export-flag" imsgcrawl contacts export --help

run_step "metadata-text" imsgcrawl metadata
run_step "metadata-json" imsgcrawl --json metadata

run_step "status-before-sync-text" imsgcrawl --archive "$archive" status
run_step "status-before-sync-json" imsgcrawl --json --archive "$archive" status
run_step "sync-text" imsgcrawl --archive "$archive" sync
run_step "sync-json" imsgcrawl --json --archive "$archive" sync
run_step "status-after-sync-text" imsgcrawl --archive "$archive" status
run_step "status-after-sync-json" imsgcrawl --json --archive "$archive" status

run_step "chats-text-default" imsgcrawl --archive "$archive" chats
run_step "chats-json-default" imsgcrawl --json --archive "$archive" chats
chats_json="$last_stdout"
run_step "chats-json-limit-one" imsgcrawl --json --archive "$archive" chats --limit 1

first_chat_id=$(jq -r '.items[0].chat_id // empty' "$chats_json" 2>/dev/null || true)
first_chat_count=$(jq -r '.items[0].message_count // empty' "$chats_json" 2>/dev/null || true)
small_chat_id=$(jq -r --argjson max "$max_all_messages" '[.items[] | select((.message_count // 0) > 0 and (.message_count // 0) <= $max)][0].chat_id // empty' "$chats_json" 2>/dev/null || true)
small_chat_count=$(jq -r --argjson max "$max_all_messages" '[.items[] | select((.message_count // 0) > 0 and (.message_count // 0) <= $max)][0].message_count // empty' "$chats_json" 2>/dev/null || true)

append_note "Selected first_chat_id=$first_chat_id first_chat_message_count=$first_chat_count small_chat_id=$small_chat_id small_chat_message_count=$small_chat_count max_all_messages=$max_all_messages."

if [[ -n "$first_chat_id" ]]; then
  run_step "messages-text-default-first-chat" imsgcrawl --archive "$archive" messages --chat "$first_chat_id"
  run_step "messages-json-default-first-chat" imsgcrawl --json --archive "$archive" messages --chat "$first_chat_id"
  run_step "messages-json-limit-three-first-chat" imsgcrawl --json --archive "$archive" messages --chat "$first_chat_id" --limit 3
else
  append_note "No chat ID was available, so message commands were skipped."
fi

if [[ -n "$small_chat_id" ]]; then
  run_step "messages-text-all-small-chat" imsgcrawl --archive "$archive" messages --chat "$small_chat_id" --all
  run_step "messages-json-all-small-chat" imsgcrawl --json --archive "$archive" messages --chat "$small_chat_id" --all
else
  append_note "No chat with 1..$max_all_messages messages was available, so messages --all was skipped."
fi

if [[ -n "$search_query" ]]; then
  run_step "search-text-limit-three" imsgcrawl --archive "$archive" search --limit 3 "$search_query"
  run_step "search-json-limit-three" imsgcrawl --json --archive "$archive" search --limit 3 "$search_query"
  if [[ "$include_search_all" -eq 1 ]]; then
    run_step "search-text-all" imsgcrawl --archive "$archive" search --all "$search_query"
    run_step "search-json-all" imsgcrawl --json --archive "$archive" search --all "$search_query"
  else
    append_note "Search --all was skipped. Re-run with --include-search-all if the query is narrow enough for a full raw transcript."
  fi
else
  run_step "search-text-empty-hit-shape" imsgcrawl --archive "$archive" search --limit 3 "imsgcrawl-agent-smoke-no-match"
  run_step "search-json-empty-hit-shape" imsgcrawl --json --archive "$archive" search --limit 3 "imsgcrawl-agent-smoke-no-match"
  append_note "No --query was supplied, so hit-search quality was not tested."
fi

run_step "contacts-export-text" imsgcrawl contacts export
run_step "contacts-export-json" imsgcrawl --json contacts export

cat >>"$review" <<'EOF'

================================================================================
AGENT REVIEW CHECKLIST
================================================================================

Read the review previews, then open exact raw files for any command where shape,
IDs, completeness, privacy, or size needs full inspection.

- Can an agent discover the useful commands from help alone?
- Does every documented command actually run?
- Do `--help`, `help COMMAND`, and command-local help agree?
- Do text and JSON modes differ intentionally, or is non-JSON secretly JSON?
- Are default limits obvious from help and visible from output shape?
- Does any default output look complete while hiding rows?
- Does text output show counts, completeness, and follow-up commands?
- Does JSON output keep a stable small envelope instead of a giant unstructured blob?
- Can IDs from `chats` be passed directly to `messages`?
- Are message/search/contact outputs human-readable enough for an agent to use?
- Are there machine-only fields, unstable IDs, hashes, or local internals that should not be agent-facing?
- Are errors useful, or only Go/SQLite/parser noise?
- Which outputs should become crawlkit-standard textproto or agent-friendly text later?
EOF

echo "review: $review"
if [[ "$inline_raw" -eq 1 ]]; then
  echo "transcript: $transcript"
fi
echo "raw: $raw_dir"
echo "commands: $commands_tsv"
echo "manifest: $manifest_jsonl"
echo "archive: $archive"

if [[ "$failures" -ne 0 ]]; then
  echo "failed commands: $failures" >&2
  exit 1
fi
