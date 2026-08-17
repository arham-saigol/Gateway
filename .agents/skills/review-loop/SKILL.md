---
name: review-loop
description: Review and fix code iteratively using the revi CLI until clean.
disable-model-invocation: true
---

# Review Loop

Iteratively review and fix code using the `revi` CLI until the review is clean or all remaining findings are invalid.

## 1. Run revi

Run `revi` matching the task scope:
- Working tree: `revi`
- Branch vs merge-base: `revi --base <branch>`
- Staged changes: `revi --staged`
- Specific commit or ref: `revi --commit <ref>`
- Specific paths: `revi <paths...>`
- Focus instructions: `revi --instructions "<focus>"`

## 2. Fix and validate

Follow the instructions in the `revi` output:
1. Fix all still-valid findings and complexity cuts.
2. Note any skipped findings with a one-sentence reason.
3. Run relevant tests, typechecks, or linters to validate changes.

## 3. Loop or finish

- **If fixes were made:** Re-run `revi` using the same scope flags and repeat.
- **Exit condition:** Stop when `revi` returns `No actionable findings or cuts.` or when all reported findings in the run are confirmed invalid.

Report the final status, a summary of fixes applied, and the rationale for any skipped findings.
