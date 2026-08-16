---
name: to-plan
description: Turn the finalized discussion into an implementation-ready plan and save it in plans/.
disable-model-invocation: true
---

# To Plan

Turn the agreed-upon task from the conversation into a self-contained implementation plan.

1. Capture the final intent, decisions, constraints, and acceptance criteria from the conversation. Inspect the relevant project files when needed to make the plan concrete. Treat later decisions as authoritative when the discussion changed direction.
2. Write the plan for an agent or engineer who does not have the conversation. Include the goal, relevant context, implementation work, and verification. Name specific files, components, or commands when known, but preserve implementation freedom where the discussion did not require a particular approach. Include open questions only when they genuinely block or change implementation.
3. Save the plan as `plans/<descriptive-kebab-case-name>.md` at the project root, creating `plans/` if needed. Do not overwrite an unrelated plan; choose a distinct name when necessary.

Finish by reporting the saved path.
