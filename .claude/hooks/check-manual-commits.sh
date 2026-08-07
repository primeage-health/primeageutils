#!/usr/bin/env bash
# SessionStart hook: detects commits that landed since the project-manager
# agent last checked (manual git usage, teammates' pushes, other AI sessions)
# and surfaces the ones NOT already reflected in ProjectTasks.md as
# additionalContext, so the assistant reconciles the ledger before starting
# new work.
#
# Correctness comes from checking whether a commit's Task ID already has a
# ledger row (grep below), not from pm-state.json being byte-exact - a commit
# can never record its own final SHA inside itself, so exact-match tracking
# alone would perpetually re-flag the agent's own trailing sync commits.
# pm-state.json is just a range optimization and is advanced unconditionally
# on every run.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
cd "$REPO_ROOT"

STATE_FILE=".claude/pm-state.json"
LEDGER="ProjectTasks.md"
CURRENT_HEAD="$(git rev-parse HEAD 2>/dev/null)" || exit 0

if [ ! -f "$STATE_FILE" ]; then
  mkdir -p .claude
  printf '{"last_synced_commit": "%s"}\n' "$CURRENT_HEAD" > "$STATE_FILE"
  exit 0
fi

LAST_SHA="$(jq -r '.last_synced_commit // empty' "$STATE_FILE" 2>/dev/null || true)"

if [ -z "$LAST_SHA" ] || [ "$LAST_SHA" = "$CURRENT_HEAD" ]; then
  exit 0
fi

if ! git cat-file -e "${LAST_SHA}^{commit}" 2>/dev/null || ! git merge-base --is-ancestor "$LAST_SHA" "$CURRENT_HEAD" 2>/dev/null; then
  # Recorded commit is unknown or not an ancestor (rebase/force-push/first run) - resync silently.
  printf '{"last_synced_commit": "%s"}\n' "$CURRENT_HEAD" > "$STATE_FILE"
  exit 0
fi

COMMITS="$(git log "${LAST_SHA}..${CURRENT_HEAD}" --pretty=format:'%h|%an|%s' 2>/dev/null || true)"

UNRECONCILED=""
while IFS='|' read -r hash author subject; do
  [ -z "$hash" ] && continue
  TASK_ID="$(printf '%s' "$subject" | grep -oE '^TASK-[0-9]+' || true)"
  if [ -n "$TASK_ID" ] && [ -f "$LEDGER" ] && grep -qF "| ${TASK_ID} |" "$LEDGER"; then
    continue
  fi
  UNRECONCILED="${UNRECONCILED}- ${hash} by ${author}: ${subject}"$'\n'
done <<< "$COMMITS"

# Always advance the marker, independent of whether anything was flagged below.
printf '{"last_synced_commit": "%s"}\n' "$CURRENT_HEAD" > "$STATE_FILE"

if [ -z "$UNRECONCILED" ]; then
  exit 0
fi

CONTEXT="The following commits landed on this repo since the project-manager agent last checked, and don't already have a matching Task ID row in ProjectTasks.md (likely manual git usage, a teammate's push, or another session):"$'\n\n'"${UNRECONCILED}"$'\n'"Before starting any new work this session, invoke the project-manager agent to inspect each commit (git show --stat) and add a ProjectTasks.md row per commit with Status=Finished and Remarks noting it was a manual/external check-in."

ESCAPED="$(printf '%s' "$CONTEXT" | jq -Rs .)"

printf '{"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": %s}}\n' "$ESCAPED"
