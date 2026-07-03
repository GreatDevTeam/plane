# PLANE - Development Process with Plane.so

Development loop using Plane.so as the task board. **The task you must work on is injected into this prompt** (see the `## Your task` section appended below) — you do not fetch it. The automation owns task selection and all Plane state transitions: it moves the task to **In Progress** before this iteration and to **Review** after it. Implement the task in a dedicated branch, open a PR, then signal completion.

## Communication rules (read first)

- **All communication goes through task comments — never address the operator in your stdout/response.** Questions, answers, status, and blockers must be posted with `docs/plane.sh add-comment <id> "<html>"`. Your textual response is not seen by anyone.
- **Everything sent to Plane must be HTML, not Markdown.** Comments and description fragments use `<p>`, `<code>`, `<a href>`, `<ul><li>`, `<strong>`, etc. Never send `**bold**`, `` `code` ``, `[text](url)`, or `- bullet` Markdown — it will not render.
  - ❌ WRONG: `add-comment <id> "[PR #57](https://github.com/x/pull/57)"` → renders as the literal text `[PR #57](...)`.
  - ✅ RIGHT: `add-comment <id> "<p><a href=\"https://github.com/x/pull/57\">PR #57</a></p>"`.
  - Before every `add-comment`, re-read the body: if it contains `[`…`](`, `**`, `` ` ``, or a leading `- `, rewrite it as HTML first.
- **If a comment or the description asks a question, answer it in a comment** (`add-comment`). Do not answer only in your response.
- **If you hit an infrastructure problem** — cannot run code, tests fail to start, code-quality tools do not run, database/Redis/ChromaDB connection errors, Docker issues, missing services — **post a comment describing the problem** (what you ran, the error) so the operator sees it, then signal completion.

## Task states (managed by the loop)

You never change task state — the loop owns every transition. Your task is **already In Progress**; the loop moves it to **Review** when the iteration ends. Do **not** call `set-in-progress`, `set-review`, `set-done`, or `set-cancelled`.

- **Re-queue on test failure:** before each iteration the loop moves any **Review** task whose PR's `Run tests in container` check **failed** back to **Todo** (build / code-quality / deploy failures do not count). A re-picked task continues on its existing branch/PR — see _Iteration detection_.
- **New sub-tasks** you create default to **Backlog** (staging); they are promoted to **Todo** manually when ready.

## Plane API Helper

All Plane interactions go through `docs/plane.sh` (run from repo root). **Comment and description bodies must be HTML.**

```bash
docs/plane.sh add-comment <id> "<html>"           # Post an HTML comment on the issue
docs/plane.sh get-comments <id>                    # List all comments [{id,body,created_at}]
docs/plane.sh get-issue <id>                       # Full issue JSON
docs/plane.sh update-description <id>              # Replace description_html (reads HTML from stdin)
docs/plane.sh append-description <id>             # Append HTML to END of description (reads from stdin)
docs/plane.sh prepend-description <id>            # Prepend HTML to START of description (reads from stdin)
docs/plane.sh set-branch <id> <branch>            # Append branch tag to description AND post a comment
docs/plane.sh set-pr <id> <pr_url>                # Append PR link to description AND post a comment
docs/plane.sh create-task <name> [desc] [priority] [backlog|todo]   # Create new task (default: backlog)
```

> **CRITICAL — `set-branch` and `set-pr` ALREADY post a comment.** Each updates the description **and** posts a comment in a single call. Call each **exactly once** and then **STOP** — do **NOT** follow it with any `add-comment` carrying the same branch/PR link, the commit message, or a "PR is ready" note. The comment is already there. A second `add-comment` is a duplicate and is forbidden.
>
> - ❌ WRONG: `set-pr <id> "$PR_URL"` immediately followed by `add-comment <id> "$(git log -1 ...)"` or `add-comment <id> "[PR #57](...)"`.
> - ✅ RIGHT: `set-pr <id> "$PR_URL"` — and nothing else about the PR.

When creating sub-tasks during implementation, use `backlog` (the default). They are not picked up until manually moved to **Todo**.

## GitHub Helper

GitHub operations go through `docs/github.sh` (wraps `gh`):

```bash
docs/github.sh pr-number <branch>            # PR number for a branch ("" if none)
docs/github.sh pr-url <branch>               # PR html URL for a branch ("" if none)
docs/github.sh pr-state <branch>             # OPEN | MERGED | CLOSED | NONE
docs/github.sh tests-status <branch>         # test check only: SUCCESS | FAILURE | PENDING | NONE
docs/github.sh unresolved-threads <branch>   # unresolved review threads as JSON [{id, body}]
docs/github.sh resolve-thread <thread_id>    # mark a review thread resolved
docs/github.sh create-pr <base> <head> <title> <body>   # create a PR, prints its URL
```

## Development Steps

### 0. Read the injected task

The task is in the `## Your task` JSON appended to this prompt. It is **already In Progress** — do not move it. Extract:

- `id` — issue UUID (used in all Plane API calls)
- `sequence_id` — integer (used in the branch name)
- `name` — task title
- `description_html` — description (HTML)
- `priority`
- `comments` — array of `{id, body, created_at}` (may be empty)

### 0.1. Sync comments to description checklist

**Always run this step — even if there appear to be no comments.**

#### 1. Collect all pending items

- **Plane task comments** — already in the injected `comments` array.
- **GitHub PR review threads** — if `description_html` contains a `Branch: <code>…</code>` tag, fetch unresolved threads:
  ```bash
  docs/github.sh unresolved-threads <branch>   # → [{id, body}]
  ```
  Keep each thread `id` — needed to resolve it later.

#### 2. Add new items to the description checklist

Read the current description and compare against checklist lines (containing `[ ]` or `[x]`). For every comment/thread body **not yet present**, append it as a new `[ ]` item (HTML):

```bash
printf '<p>[ ] <new item text></p>' | docs/plane.sh append-description <id>
```

If there is no checklist yet, append a heading first:

```bash
printf '<hr/><p><strong>Checklist:</strong></p><p>[ ] <item></p>' | docs/plane.sh append-description <id>
```

#### 3. After implementing each checklist item

Mark it done in the description and (for PR threads) resolve the GitHub conversation:

```bash
# Mark done in description (rewrite the full HTML)
UPDATED=$(docs/plane.sh get-issue <id> | jq -r '.description_html' | sed 's/\[ \] fix X/[x] fix X/')
printf '%s' "$UPDATED" | docs/plane.sh update-description <id>

# Resolve the GitHub thread (if this item came from a PR review)
docs/github.sh resolve-thread <thread_id>
```

#### 4. Detect merge instruction

Scan every comment body (Plane + PR) for `merge with <branch>`, `merge to <branch>`, or `merge into <branch>` (case-insensitive). If found, note the target branch — used in step 5.1.

#### 5. Answer questions in comments

If any comment or the description poses a question you can answer, **answer it in a comment** before or during implementation:

```bash
docs/plane.sh add-comment <id> "<p>Answer: …</p>"
```

### 1. Create a git branch

If `description_html` mentions a specific branch (e.g. "implement in branch X" or "branch: X"), use that name. Otherwise generate one.

Branch name format: `feature/{sequence_id}_{name_slug}`

Rules for `name_slug`: lowercase the name; spaces → hyphens; remove non-alphanumeric except hyphens; collapse repeated hyphens; strip leading/trailing hyphens; truncate to 50 chars at a word boundary.

```bash
git checkout dev
git pull origin dev
git checkout -b <branch>
docs/plane.sh set-branch <id> <branch>
```

#### Iteration detection

If the description already contains a `Branch: <code>…</code>` tag (written by `set-branch` in a prior iteration), this task is continuing. Detect the existing PR state:

```bash
BRANCH=$(docs/plane.sh get-issue <id> | jq -r '.description_html' | grep -oP '(?<=Branch: <code>)[^<]+' | tail -1)
PR_STATE=$(docs/github.sh pr-state "$BRANCH")
```

- **`OPEN`** — check out the existing branch and continue; **do not create a new branch or call `set-branch`**:
  ```bash
  git fetch origin
  git checkout "$BRANCH"
  ```
- **`MERGED` or `NONE`** — a new PR is needed. Create a new branch by appending `-v2` (then `-v3`, …, until unused):
  ```bash
  NEW_BRANCH="${BRANCH}-v2"
  git checkout dev
  git pull origin dev
  git checkout -b "$NEW_BRANCH"
  docs/plane.sh set-branch <id> "$NEW_BRANCH"
  ```

### 3. Investigate and implement

3.1. Read `name` and `description_html` to understand the task.

3.2. **If the description is short** (no checklist, no investigation notes, no clear subtasks) — investigate the relevant code first, then write findings back into the description before touching code:

```bash
CURRENT_HTML=$(docs/plane.sh get-issue <id> | jq -r '.description_html // ""')
printf '<hr/><p><strong>Investigation:</strong></p><p>…</p><p><strong>Checklist:</strong></p><p>[ ] subtask 1</p><p>[ ] subtask 2</p>' | docs/plane.sh append-description <id>
```

If questions surface during investigation, **post them as a comment** and stop:
```bash
docs/plane.sh add-comment <id> "<p>Question: …</p>"
```
```
<promise>TASK_DONE</promise>
```

If no questions, continue to implementation using the checklist you just wrote.

3.2.1. **If the task is purely technical** (names a class/method/file/config to change without business context) and investigation reveals missing context needed to implement correctly (unclear API contract, unknown callers, undescribed integration point), post the specific blockers as a comment and stop:

```bash
docs/plane.sh add-comment <id> "<p>Technical blockers:</p><ul><li>…</li></ul>"
```
```
<promise>TASK_DONE</promise>
```

3.3. Investigate the relevant code (if not done in 3.2).
3.4. If questions arise before writing code, post them as a comment and stop with `<promise>TASK_DONE</promise>`.
3.5. Implement following all project rules in `CLAUDE.md`. After each checklist item, mark it done in the description (step 0.1 #3).
3.6. Add or update tests for changed functionality.

### 3.7. Investigate production errors via Elasticsearch (logs)

Use `docs/elastic.sh` to query live logs (`http://10.10.0.27:9200`, no auth). Fields: `timestamp [level] [instance] [code] [website]  message`.

```bash
docs/elastic.sh errors [N] [--instance us|uk|de|fr|pl|ca|...] [--code php|python] [--website jobscanner|talentmediator]
docs/elastic.sh search --query 'level_name.keyword:"ERROR" AND NOT message:"Jobs found"' [N]
docs/elastic.sh tail [N] [--level ERROR|WARNING|INFO] [--instance X] [--code X] [--website X]
docs/elastic.sh stats [1h|24h|7d]
docs/elastic.sh indices
```

### 4. Run tests and quality gates

**4.1. Run tests for every Django app with changed files.** Identify changed apps with `git diff --name-only HEAD`, then for each app run `make run "python manage.py test <app>"`. All tests must pass.

**4.2. Run code quality checks:**
```bash
make run "mypy ."
make run "bandit -q -r . -c pyproject.toml -ll -ii"
```

Fix all reported issues. **If a test or tool cannot run at all** (Docker/infra/connection failure rather than a code defect), post a comment describing it (step "Communication rules") and stop.

### 5. Commit and push

**Never push to `dev` or `master`.** Always push to the feature branch.

```bash
git add -p
git commit -m "feat: <short description>

- Detail 1;
- Detail 2."
git push origin <branch>
```

### 5.1. Merge into target branch (only if a merge instruction was found in step 0.1)

```bash
git checkout <target_branch>
git pull origin <target_branch>
git merge <feature_branch>
git push origin <target_branch>
git checkout <feature_branch>
```

Then continue to step 6.

### 6. Create PR and record it on the task

```bash
PR_URL=$(docs/github.sh create-pr dev <branch> "<task name>" "Plane task: <sequence_id>")
docs/plane.sh set-pr <id> "$PR_URL"
```

`set-pr` appends the PR link to the description **and** posts a comment — that is the **single, complete** command for recording the PR. After it runs, recording the PR is **done**.

> **Do NOT** add any further comment about the PR — no `add-comment` with the PR link, the commit message, or a "ready for review" note. `set-pr` already posted the comment; anything more is a duplicate. Two commands only at this step:
> ```bash
> PR_URL=$(docs/github.sh create-pr dev <branch> "<task name>" "Plane task: <sequence_id>")
> docs/plane.sh set-pr <id> "$PR_URL"   # ← last PR-related command; STOP here
> ```

> The loop calls `set-review` automatically after the iteration — do not call it.

### 7. Post-task analysis

Before signalling done, reflect on what knowledge was **missing from `CLAUDE.md` or the skills** that would have made this task easier (an undocumented pattern, a missing helper command, an architectural rule, a `PLANE.md` ambiguity). If anything significant is missing, post a comment with specific suggestions (HTML). If everything was available, skip this.

```bash
docs/plane.sh add-comment <id> "<p>CLAUDE.md / skills suggestions:</p><ul><li>…</li></ul>"
```

### 8. Cleanup

Stop only this project's containers:

```bash
docker compose -f docker-compose.ralph.yml ps -q | xargs -r docker stop
```

### 9. Signal completion

Output:
```
<promise>TASK_DONE</promise>
```

## General rules

- **When mentioning code in comments/descriptions, link to it on GitHub** using HTML anchors and full permalinks:
  `<a href="https://github.com/GreatDevTeam/plane/blob/<branch>/<path>#L<line>">UserService.handle()</a>`

## Signals

| Signal | Meaning |
|--------|---------|
| `<promise>TASK_DONE</promise>` | Iteration complete — loop posts stats, moves the task to Review, picks the next task |

The loop moves the task to **Review** at the end of the iteration **whether or not** you emit `TASK_DONE`. Emit `TASK_DONE` to start a fresh session for the next task.

## Commit rules

### Format
```
type: short description

- Detailed point 1;
- Detailed point 2.
```

### Types
`feat` · `improvement` · `fix` · `refactor` · `docs` · `test` · `chore`

### Rules
- Do NOT add "Generated with Claude Code" or similar attribution
- Do NOT add "Co-Authored-By" lines
