# PR Replica Selector Design

**Goal**

Create a reusable Codex skill that scans historical PRs from a ByteDance internal repository via `bytedcli`, filters out poor candidates, and ranks the remaining PRs by how suitable they are for a junior developer intern to replicate and discuss on a resume.

**Non-goals**

- Do not generate implementation plans for recreating the selected PRs.
- Do not auto-modify code in the target repository.
- Do not depend on a single hard-coded `bytedcli` subcommand that may drift; keep the workflow tool-first and command-discovery-first.

**Primary User**

A junior developer intern who wants a large batch of realistic historical requirements to replicate, but only when the requirement is feasible, mostly self-contained, and strong enough to discuss in a resume or interview.

**Design Choices**

1. Use a workflow-first skill instead of a prompt-only skill.
   This keeps the screening criteria stable across large batches.
2. Depend on `bytedcli` for internal repository and PR access.
   The skill should route the agent to `bytedcli` and explicitly avoid guessing internal APIs.
3. Use explicit exclusion rules before scoring.
   This reduces wasted effort on obviously unsuitable PRs.
4. Use a weighted rubric oriented around intern feasibility and resume value.
   This matches the user's actual decision criteria better than generic code quality scoring.

**Inputs**

- Repository URL, repo identifier, or local checkout context
- Optional time range
- Optional path scope, team scope, or keyword scope
- Optional desired batch size

**Output Shape**

For each candidate PR, produce:

- PR identifier and title
- Link or retrieval handle
- Overall recommendation
- Per-dimension scores
- Why it is suitable
- Why it may not be suitable
- Resume suitability judgment
- Main risks or hidden costs
- Priority tier

**Filtering Rubric**

Exclude PRs that are likely poor replication targets:

- Obvious multi-owner or cross-system coordination work
- Heavy online permissions, private data, or environment dependencies
- Urgent incident patches with weak generalizable value
- Pure refactors, config churn, or migration plumbing with weak demo value
- Changes that appear to require deep hidden business knowledge
- Architecture work clearly beyond expected intern ownership

Score remaining PRs on:

- Technical complexity is challenging but bounded
- One intern could plausibly complete most of the work
- Dependencies are limited and understandable
- Business context can be recovered without deep insider knowledge
- Diff scope is focused enough to reproduce
- Resume value is easy to explain
- Result is demonstrable within limited time
- Environment and permissions are realistically obtainable

**Resources**

- `SKILL.md`: workflow, exclusion rules, scoring rubric, output template
- `references/rubric.md`: detailed interpretation of each score dimension
- `references/output-template.md`: stable candidate output format
- `references/bytedcli-usage.md`: lightweight guidance on how to use `bytedcli` safely in this workflow

**Validation**

- Validate skill folder structure with `quick_validate.py`
- Keep the skill lightweight and deterministic in shape
- Defer forward-testing against the live internal system until a real retrieval run is needed
