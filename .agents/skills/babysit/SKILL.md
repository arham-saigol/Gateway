---
name: babysit
description: Monitor and repair the current pull request until its latest commit has no valid review-bot findings.
disable-model-invocation: true
---

# Babysit

Stay with the current PR through bot review, fixes, and follow-up reviews. Poll GitHub through the installed `gh` CLI; keep unchanged payloads out of the conversation.

## Establish the head

1. Verify `gh auth status`, identify the open PR for the current branch, and record `{owner}/{repo}`, PR number, head SHA, and head commit time. Stop with the cause when authentication or an open PR is missing.
2. Take an initial snapshot with `gh api`, paginating where needed:
   - `repos/{owner}/{repo}/pulls/{number}`;
   - `repos/{owner}/{repo}/pulls/{number}/reviews`;
   - `repos/{owner}/{repo}/pulls/{number}/comments`;
   - `repos/{owner}/{repo}/issues/{number}/comments`;
   - `repos/{owner}/{repo}/issues/{number}/reactions`.
3. Record event IDs, timestamps, actors, review `commit_id`s, and the head SHA to which each signal belongs.

## Run a quiet watcher

Use one long-lived background polling process when the harness supports it. Have it call `gh api`, canonicalize the relevant fields, and emit or exit only when the head or snapshot digest changes. Avoid repeated assistant turns for sleeping or unchanged API payloads.

Poll quickly while a review is active, then back off while waiting for a service to start. Honor GitHub API rate-limit headers and retry after the reset time. A review bot reporting that it is rate-limited is a service result, not a reason to poll GitHub more aggressively.

On every meaningful change:

1. Refresh the PR first. If `head.sha` changed, discard completion decisions from the old head, retain handled event IDs, and begin a new head cycle.
2. Re-fetch reviews, review comments, issue comments, and reactions.
3. Classify each bot for the current head as **pending**, **clean**, **finding**, or **unavailable/stale**.

## Interpret review signals

Tie a signal to the current head by `commit_id` where GitHub supplies one. For issue comments and other SHA-less signals, require that they were created after the head commit and clearly concern the current revision. An old approval cannot clear a newer commit.

Treat findings from review bots such as CodeRabbit, Greptile, and Codex as leads to verify, not ground truth. An explicit no-findings result or approval marks that bot clean. A rate-limit notice or a statement that the bot reviews only the first commit marks it unavailable/stale for this head; it neither blocks forever nor counts as clean. Ignore duplicate and superseded events.

### Codex

Codex reports its state through reactions on the PR rather than GitHub checks. Count only reactions made by the Codex bot identity:

- `eyes` created after the head commit means the current review is pending;
- `+1` created after the head commit means the review completed with no findings and is clean;
- current-head Codex review comments or review bodies containing findings mean finding.

When both reactions exist, use their timestamps and subsequent Codex events to determine the latest state. Human reactions and reactions predating the head are not Codex completion signals.

## Handle findings

Verify every unseen finding against the current checkout and latest head. For each valid finding, make the smallest coherent fix that addresses the demonstrated problem without unrelated cleanup. Run focused validation for the changed behavior and any repository-required validation relevant to the edited files.

Skip invalid, duplicate, already-fixed, or stale findings. Record a concise evidence-based reason for each skip.

When code changes, commit only the fixes, push, and begin a fresh head cycle. Every push invalidates prior clean review signals. Resolve addressed review threads when permissions and certainty allow.

## Exit gate

Finish only when all of these hold for the same latest SHA:

- no unresolved valid current-head bot finding remains;
- Codex has a current-head `+1` clean signal, unless it explicitly reported itself unavailable for that head;
- every other bot that started a current-head review is clean or explicitly unavailable/stale; and
- one final poll after the candidate clean state finds the head and review snapshot unchanged.

Summarize the final SHA, valid findings fixed and their validation, and findings skipped with the reason for each skip.
