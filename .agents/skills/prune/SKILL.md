---
name: prune
description: Review code for over-engineering and identify what to cut or simplify. Use when the user asks to prune or simplify code.
---

# Prune

Review the requested code for unnecessary complexity. The best outcome is fewer lines, fewer concepts, and fewer dependencies with the same required behavior. Report cuts only; do not edit the code.

## 1. Set the boundary

Use the scope supplied by the user. For current work, inspect staged, unstaged, and untracked changes. For a branch or ref, compare its merge-base with `HEAD`. For supplied files or snippets, inspect that material and only enough surrounding code to verify each cut.

Read repository instructions and relevant tests before judging whether complexity is required. This step is complete when every file and hunk in scope is known.

## 2. Hunt subtraction

Inspect every in-scope region for:

- `delete:` dead code, duplicated logic, speculative features, defensive paths for impossible states, or comments that restate the code;
- `inline:` a wrapper, helper, class, hook, service, or layer with one caller and no independent contract;
- `stdlib:` hand-written behavior already provided by the language, standard library, framework, or an existing project utility;
- `dependency:` a package used for behavior the platform or a few clear lines already provide;
- `yagni:` configuration nobody varies, generic types with one concrete use, extension points with one implementation, or flexibility unsupported by a current requirement;
- `collapse:` parallel types, states, branches, or transforms that represent the same concept;
- `shrink:` verbose control flow or data manipulation with a shorter, equally readable form.

Treat an abstraction as a cost that must buy a real boundary, repeated use, hidden complexity, or independent change. Prefer direct code when it buys none of these.

Preserve required behavior, public contracts, useful tests, and checks at real trust boundaries. Do not propose a cut based only on personal style or a hypothetical future redesign.

This step is complete when every in-scope region has been checked and each retained finding removes a concrete line, concept, layer, or dependency.

## 3. Prove the cut

Keep a finding only when:

1. the reported code exists at the current location;
2. repository evidence shows the complexity is unnecessary;
3. the simpler replacement is concrete and preserves required behavior;
4. the cut is local enough to describe without inventing a new architecture;
5. the estimated line reduction is defensible.

Discard uncertain findings. This is a pruning pass, not a correctness, security, performance, or formatting review.

## 4. Report

Group cuts by file using this format:

```text
Cuts:
In `@path/to/file.ts`:
- Around line 20-45: `inline` Remove the single-use wrapper. Call the function directly. (-18 lines)
- Line 72: `stdlib` Remove the custom lookup. Use `Map.get`. (-6 lines)

In `@path/to/another-file.ts`:
- Around line 10-16: `delete` Remove the unused configuration branch. Nothing replaces it. (-7 lines)
```

Use current working-tree line numbers and a single line number when appropriate. Order file groups by their largest cut, then order cuts within each file by estimated reduction. Be direct; do not add praise, caveats, severity labels, correctness findings, or a summary of the code.

Count only reductions from reported cuts. When returning the review in chat, end with:

`Net: -<N> lines possible.`

When writing the cuts to a Markdown file or any other saved report or artifact, omit the net line from that artifact. Give the net line in the normal chat response instead.

If there is nothing worth cutting, return only:

`Lean already. Ship.`