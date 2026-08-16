---
name: to-prompt
description: Turn the finalized discussion into a prompt for a capable autonomous agent and save it in prompts/.
disable-model-invocation: true
---

# To Prompt

Turn the agreed-upon task from the conversation into a self-contained execution prompt.

1. Recover the final intent, decisions, constraints, and acceptance criteria. Treat later decisions as authoritative when the discussion changed direction. Inspect relevant project files only when needed to make the prompt accurate. The receiving agent will not have this conversation.
2. Write for a highly capable autonomous agent. Define the destination: the outcome, essential context, settled decisions, real constraints, completion bar, and material verification. State action or approval boundaries when they matter. Give the agent room to investigate, choose methods, use its tools, and make in-scope decisions without waiting for instructions.
3. Keep the prompt proportional to the task. Include a fact or instruction only when it changes execution. Prefer decision criteria over exhaustive rules and specific outcomes over prescribed steps. Specify files, commands, formats, or sequencing only when known or genuinely required. Preserve unresolved implementation freedom instead of deciding it for the agent.
4. Make the prompt executable rather than explanatory. Lead with the task. Add only the context the receiving agent cannot discover efficiently. Use a completion criterion the agent can verify, including relevant tests or checks. Ask for user input only where missing information truly blocks the work or an action crosses the prompt's approval boundary.
5. Prune generic role-play, motivational language, tutorials, repeated requirements, speculative edge cases, and workflow narration. Do not turn the prompt into an implementation plan or tell the agent how to reason. The final prompt must be concise, direct, and sufficient for the agent to complete the work end to end.
6. Save only the prompt as `prompts/<descriptive-kebab-case-name>.md` at the project root, creating `prompts/` if needed. Do not overwrite an unrelated prompt; choose a distinct name when necessary.

Finish by reporting the saved path.
