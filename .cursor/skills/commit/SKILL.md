---
name: commit
description: commit this chat, git commit agent output, snapshot chat changes, commit work from this conversation, commit only this session's changes, split commits from this chat
agent-config-sync: true
---

# Commit changes from an agent chat

## Goal

Turn edits from **this conversation** into one or more **clean commits**: correct staging, messages that match project style if obvious, and **logical splitting** when multiple independent changes landed in **this chat**.

**Default scope:** Only paths you can tie to **the current chat** (or a transcript/session the user explicitly points you at *for this run*). **Do not** stage or commit other dirty files just because `git status` lists them - those may be from another chat, manual work, or another branch of work unless the user clearly says otherwise.

## Preconditions

- Shell can run `git` and `gh` from the relevant repository root.
- If commits will be pushed or opened as a PR, ensure `gh auth status` succeeds when that step is in scope.

## Workflow

### 1. Locate the repo and inventory changes

Use `git status` / `git diff` to see the full working tree, but treat that listing as **inventory only**. You will stage **only** the subset that belongs to this chat (next section) - not â€œeverything changed.â€

From the workspace (or path the user gives), run:

```bash
git rev-parse --show-toplevel
git status -sb
git diff
git diff --staged
```

Optional GitHub context (remote, default branch) without leaving the terminal:

```bash
gh repo view --json nameWithOwner,defaultBranchRef,url
```

### 2. Map changes to the chat (mandatory filter)

1. **Attribution:** Decide which paths were **actually produced or intentionally modified in this chat** (or in a **user-supplied** transcript/session the user asked you to use for this commit). Use the conversation: files touched, tasks completed, explicit user requests, tool edits traceable to this thread.
2. **`git diff` confirms content**, not inclusion: a file appearing in `git diff` does **not** mean it belongs in this commit if this chat never discussed or changed it.
3. **Exclude by default:** Leave untouched in the working tree (unstaged) any changed or untracked paths **not** attributable to this chatâ€™s scope - **even if** the user would like a â€œcleanâ€ status. They must **explicitly** ask to commit â€œeverything,â€ â€œall local changes,â€ named paths, or work from another context before you widen what you `git add`.
4. If it is **unclear** whether a path belongs to this chat, **ask one short question** or commit only the obvious subset and mention what was left out.
5. If the user points at a transcript or session, use it only to recall *which* files and *what* themes belong to *that* scope; still verify with `git diff`.

### 3. Decide one commit vs several

Apply this **only to paths you already attributed to this chat** in step 2 - not to the whole repo diff.

**Prefer a single commit** when:

- The diff is small, or
- All edits serve one intent (one bugfix, one feature slice, one doc update).

**Split into multiple commits** when:

- Unrelated concerns are mixed (e.g. refactor + bugfix + formatting sweep),
- Changes would be hard to revert or cherry-pick as one unit,
- Different areas would benefit from different message scopes (`feat` vs `fix` vs `docs`).

Order commits **dependency-first** (lower-level or shared helpers before call sites).

Briefly tell the user the plan (e.g. "2 commits: fix X, then docs for Y") before running commands.

### 4. Stage and commit with git

For each commit:

1. Stage only the paths that belong to that logical change **and** to this chatâ€™s scope: `git add -- path1 path2` (avoid `git add .` unless every dirty path is intentionally in scope for this commit).
2. Re-check: `git diff --staged`
3. Commit with a message that matches obvious repo conventions; if none, use a short imperative subject and optional body (Conventional Commits is a safe default).

```bash
git commit -m "type(scope): short summary" -m "Optional body with rationale."
```

Repeat until **chat-attributed** work is committed. **Expect** other local changes to remain modified or untracked - that is normal when scope is chat-only.

### 5. Use `gh` after commits (when relevant)

`gh` does not replace `git` for staging/commits. Use it when the user wants GitHub next steps, for example:

- `gh pr create` after pushing a branch
- `gh issue view <n>` to reference an issue in the PR or commit message

Do not push or open a PR unless the user asks.

## Guardrails

- Skip secrets, `.env`, credentials, and large generated artifacts. Flag them if they appear in `git status`.
- Stage and commit only paths attributable to **this chat** (or a user-named transcript for this run). Leave unrelated dirty files unstaged unless the user explicitly widens scope.
- Avoid force-push and history rewrites unless the user explicitly requests them.
