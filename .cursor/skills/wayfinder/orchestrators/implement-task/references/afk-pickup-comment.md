# AFK pickup comment

Post when an AFK implementation task becomes eligible for **implement-task** pickup. Used by [create-tasks](../../../actions/create-tasks/SKILL.md) on **`tasks approved`** and by **implement-task** when unblocking dependents or serial handoff to the next AFK task.

## Trigger phrase (canonical)

```text
Approved - AFK implement
```

| Role | Detail |
|------|--------|
| **Automation trigger (v1)** | Cursor repo automation: **issue comment** containing this exact phrase (case-sensitive) |
| **Human signal** | Label **`wf:approved`** on the task - reviewer visibility; implement-task startup gate |
| **Future** | When Cursor supports **issue label added** on **`wf:approved`**, automation may switch to label-only; keep posting this comment until all app repos migrate |

Do **not** use **`tasks approved`**, bare **`approved`**, or Reconcile phrases (**`Approved - reconcile and close`**) as the AFK pickup trigger - those gate different skills.

## When to post

Post **only** when **all** of the following hold:

1. Task has label **`wf:afk`** (not HITL)
2. Task **Status:** `ready`
3. **Blocked by** cleared (empty, or every blocker **closed** / **`awaiting-reconcile`**)
4. Agent adds (or confirms) label **`wf:approved`** on the same pickup decision

| Owner | Moment |
|-------|--------|
| **create-tasks** | On **`tasks approved`** when adding **`wf:approved`** to the one eligible AFK task |
| **implement-task** | After success resolution when a dependent becomes unblocked |
| **implement-task** | AFK serial handoff - next eligible task in queue after removing **`wf:afk-running`** from the finished task |

**One comment per pickup decision** - do not repost on re-runs unless human explicitly resets pickup.

## Comment template

Post as a **new top-level comment** on the AFK task. The trigger phrase must appear **verbatim on its own line** so automation filters match reliably.

```markdown
## AFK implement pickup

**Approved - AFK implement**

Task **Status:** `ready` - label **`wf:approved`** added for reviewer visibility.
**Method:** `{method-name}` - bundle branch from parent **Branch:** line.

<!-- Automation: issue comment trigger v1. Future: wf:approved label add may replace comment trigger. -->
```

Replace `{method-name}` with the task **## Method** value. Omit the HTML comment when posting via `gh issue comment` if your team prefers a clean thread - automation needs only the trigger line.

## gh example

```powershell
gh issue comment <task-num> --body-file path\to\afk-pickup.md
```

```bash
gh issue comment <task-num> --body-file path/to/afk-pickup.md
```

## Pair with label

Always add **`wf:approved`** in the same pickup decision (before or after the comment):

```powershell
gh issue edit <task-num> --add-label "wf:approved"
```

HITL tasks (**`wf:hitl`**) never receive this comment - human starts implement-task in chat instead.
