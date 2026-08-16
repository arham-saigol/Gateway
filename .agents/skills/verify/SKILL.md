---
name: verify
description: Review and simplify the current work, fix every finding, and validate the result.
disable-model-invocation: true
---

# Verify

1. Run `/review` and `/prune` independently against the same pre-fix scope. Run them in parallel when possible; otherwise, complete both before editing.
2. Implement the smallest coherent changes that address every finding and cut. Reconcile overlaps in favor of required behavior and simpler code.
3. Run focused checks for the changed behavior, then the repository's required validation.

Finish only when every finding is addressed and validation passes. Report the fixes and validation results.
