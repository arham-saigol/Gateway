---
name: review
description: Review code for actionable defects. Use when the user asks for a code review, PR review, branch or diff review, review of current changes, or review since a commit, branch, tag, or merge-base. Verify findings against the current code and report only fix-ready issues and worthwhile nitpicks; do not edit code unless the user also asks for fixes.
---

# Review

Act as the engineer who must defend every finding to the author and hand it to another engineer to fix. Review the requested change, not the idea of the change. Evidence outranks suspicion.

## 1. Pin the review scope

Honor the scope and comparison point that the user supplied. For a branch or ref, compare its merge-base with `HEAD`. For current work, inspect staged, unstaged, and untracked files. For supplied files or snippets, review only that material and the surrounding code needed to understand it.

Read the repository instructions and the relevant issue, specification, tests, and documentation when available. If the intended comparison cannot be inferred safely, ask one concise question before reviewing.

This step is complete when every file and hunk in scope is known, including changes that ordinary diff output can miss.

## 2. Establish the contract

Read each changed region in its current file. Trace callers, callees, types, data flow, error paths, state transitions, and platform boundaries far enough to determine the intended behavior. Use tests and documentation as evidence, while allowing for the possibility that either is stale.

For each change, identify the behaviors that must remain true and the new behavior it introduces. Pay particular attention to:

- correctness at boundaries, empty states, ordering, retries, partial failure, and concurrency;
- security and privacy at trust boundaries, authorization checks, validation, secrets, and unsafe interpolation;
- compatibility of public APIs, persisted data, configuration, protocols, migrations, and supported platforms;
- resource lifetime, cleanup, transactions, cancellation, timeouts, and error propagation;
- performance only where the changed execution path creates a concrete scaling or latency problem;
- repository rules that tools do not already enforce;
- missing tests only when a specific untested behavior can regress because of this change.

This step is complete when every changed behavior has a stated contract and every affected boundary has been inspected.

## 3. Prove each candidate

A finding is admissible only when all of these are true:

1. It still exists in the current file, at the reported location.
2. It is introduced by the review scope, or the change directly makes a pre-existing defect reachable or more harmful. Report pre-existing defects only when the user requests a full audit.
3. A concrete input, state, execution path, or documented rule demonstrates the failure. Repository evidence must support claims about APIs, language behavior, and project conventions.
4. The impact is material and the smallest safe direction for a fix is known.
5. The finding does not depend on taste, an unsupported assumption, or code outside the available evidence.

Reproduce the behavior or run the narrowest relevant check when practical. Otherwise, trace it through the code and state the triggering conditions in the finding. Resolve uncertainty through inspection; omit candidates that remain speculative.

Classify an admissible issue as a **Finding** when it can cause incorrect behavior, a vulnerability, data loss, a compatibility break, a meaningful reliability or performance regression, or a concrete maintenance hazard. Classify a small, valid, local improvement as a **Nitpick**. Formatting enforced by tools, personal preferences, vague cleanup, and speculative redesign are not reportable.

This step is complete when every retained item passes all five gates and every rejected candidate has been discarded.

## 4. Write a fix-ready report

Order findings by impact, then by file and line. Group multiple items in the same file under one file heading. Use current working-tree line numbers: `Line N` for one line and `Around line N-M` for a region.

Each item must contain the defect, its trigger, its consequence, and the required behavior of the fix. Name relevant symbols or values when they make the issue reproducible. Give another agent run all reasoning needed to verify and fix the issue without access to private reviewer context. Keep implementation details open when more than one minimal fix is valid.

Return only this format:

```text
Findings:
In `@path/to/file.ts`:
- Around line 120-140: Describe the issue clearly. Explain what is wrong, why it matters, and what the fix should do. Make the finding specific enough that another Codex run can fix it without needing the reviewer's hidden reasoning.

In `@path/to/another-file.ts`:
- Around line 40-54: Describe the issue clearly. Keep it grounded in the current code. Avoid vague comments.

--

Nitpicks:
In `@path/to/file.ts`:
- Line 18: Describe a small but valid improvement.
- Line 19: Describe another small improvement in the same file.
```

Replace the examples with actual results. When a section has no items, omit it. Do not add a summary, score, praise, severity labels, or commentary outside the template.

The review is complete only when every in-scope hunk has been examined, every reported location matches the current file, every finding is independently actionable, and the report follows the template exactly.