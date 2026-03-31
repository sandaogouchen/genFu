# PR Replica Selector Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Codex skill that uses `bytedcli` to retrieve historical PRs and rank which ones are best for a junior intern to replicate.

**Architecture:** Create a new skill in `~/.codex/skills/pr-replica-selector` with a workflow-oriented `SKILL.md` and a small `references/` set. Keep the implementation tool-agnostic inside the skill except for routing through `bytedcli`, and encode the screening logic in explicit exclusion and scoring sections.

**Tech Stack:** Markdown skill files, local skill scaffolding scripts, Codex skill metadata generator, `bytedcli` as the downstream internal tool.

---

### Task 1: Scaffold The Skill

**Files:**
- Create: `~/.codex/skills/pr-replica-selector/SKILL.md`
- Create: `~/.codex/skills/pr-replica-selector/agents/openai.yaml`
- Create: `~/.codex/skills/pr-replica-selector/references/`

**Step 1: Initialize the skill directory**

Run the provided initializer with interface metadata and a `references/` directory.

**Step 2: Verify the scaffold exists**

Inspect the generated files and confirm the expected folder structure.

### Task 2: Write The Skill Workflow

**Files:**
- Modify: `~/.codex/skills/pr-replica-selector/SKILL.md`

**Step 1: Replace template frontmatter**

Write a trigger description that clearly states the skill is for bulk screening of historical PRs for intern-friendly replication and resume usefulness.

**Step 2: Add workflow sections**

Document the retrieval flow, exclusion rules, scoring rubric, and final output requirements.

**Step 3: Add operational constraints**

State that the skill should use `bytedcli`, avoid guessing internal APIs, avoid over-trusting title-only judgments, and prefer evidence from PR body, changed files, and ownership signals.

### Task 3: Add Reference Material

**Files:**
- Create: `~/.codex/skills/pr-replica-selector/references/rubric.md`
- Create: `~/.codex/skills/pr-replica-selector/references/output-template.md`
- Create: `~/.codex/skills/pr-replica-selector/references/bytedcli-usage.md`

**Step 1: Add rubric guidance**

Define each evaluation dimension and what high or low scores mean.

**Step 2: Add output template**

Provide a stable structure for ranked candidate lists.

**Step 3: Add `bytedcli` usage guidance**

Capture the minimum retrieval principles needed by this skill without duplicating the full `bytedcli` skill.

### Task 4: Validate

**Files:**
- Validate: `~/.codex/skills/pr-replica-selector`

**Step 1: Run quick validation**

Run the validator and fix any metadata or structure issues.

**Step 2: Summarize usage**

Provide a short example prompt showing how to invoke the skill.
