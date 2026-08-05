# Plane — Development Patterns

## Project Structure

This is a monorepo (pnpm workspaces + Turborepo). Frontend lives in `apps/web`, shared UI in `packages/ui`, shared state in `packages/shared-state`.

### The CE/EE extension seam (`@/plane-web/*` → `apps/web/ce/*`)

This repo is the community edition of Plane. Everything upstream gates behind a paid edition is imported through `@/plane-web/...`, which `apps/web/tsconfig.json` maps to `./ce/*` — so the call sites in `apps/web/core/` are the real upstream ones, already wired into the board, card, spreadsheet, create/update modal, detail sidebar, filter bar and activity feed, and the file under `apps/web/ce/` they resolve to is a **no-op stub** returning `null` / `<></>` / `{}`.

The practical consequence: a feature that looks like "build it from scratch" is usually "fill in the stub". Before designing anything, grep for the feature under `apps/web/ce/` — if a stub exists, implement it there and the feature lights up at every call site without touching core rendering code. Conversely, a component under `ce/` that returns an empty fragment is not dead code; deleting it breaks the import.

Three closed unions are the gate on anything that needs a per-field filter or column, and they have to be widened before a stub can do useful work: `WORK_ITEM_FILTER_PROPERTY_KEYS` (`packages/types/src/view-props.ts`), `EXTENDED_FILTER_FIELD_TYPE` (`packages/types/src/rich-filters/field-types/extended.ts` — the designated slot for non-core filter editors), and `IIssueDisplayProperties` / `ISSUE_DISPLAY_PROPERTIES_KEYS`. Server side the equivalent gate is `IssueFilterSet`.

## Development Commands

- `pnpm dev` — Start all dev servers (web:3000, admin:3001)
- `pnpm build` — Build all packages and apps
- `pnpm check` — Run all checks (format, lint, types)
- `pnpm check:lint` — OxLint across all packages
- `pnpm check:types` — TypeScript type checking (see _Type checking_ below — build the workspace packages first)
- `pnpm fix` — Auto-fix format and lint issues
- `pnpm turbo run <command> --filter=<package>` — Target specific package/app

### Type checking

`tsc` resolves the `@plane/*` workspace packages through their built `dist/`, so a bare `tsc --noEmit` in an app reports hundreds of bogus `TS2307 Cannot find module '@plane/…'` errors (plus the `TS7006` implicit-`any` cascade they drag in) whenever a package has not been built. Build the app's workspace dependencies first:

```bash
pnpm turbo run build --filter='web^...'   # web's deps only, not web itself (~45 s cold, seconds when cached)
cd apps/web && pnpm check:types
```

`apps/web`'s `check:types` is `react-router typegen && tsc --noEmit` — the typegen step is required too, otherwise every route file fails on its generated `./+types/<route>` import. Run it via `pnpm check:types` rather than calling `tsc` directly. Expect ~2–3 min; a clean run prints nothing but a `MODULE_TYPELESS_PACKAGE_JSON` warning about `packages/tailwind-config/postcss.config.js`.

### Formatting

`fix:format` runs oxfmt over the whole app, so it reformats files that were already drifting in `master` as well as the ones you touched. After running it, check `git diff --stat` and revert any file your change did not touch. Simpler: run `npx oxfmt <the files you changed>` from the repo root instead.

### The pre-commit hook denies warnings

`git commit` runs husky → lint-staged → `oxlint --fix --deny-warnings` over the **staged files only**. `pnpm check:lint` passes the repo at ~990 warnings, so a clean `check:lint` does **not** mean the commit will go through: touching a file that already carried a warning fails it, with a diagnostic about code you never wrote.

Silence those with `// oxlint-disable-next-line <rule> -- <why>` on the line the diagnostic's **primary span** starts at (not necessarily the line the message is about — `no-duplicate-enum-values` points at the _first_ member sharing the value, so an `eslint-disable` on the duplicate does not suppress it). The rule name is the part in brackets: `oxc(no-map-spread)` → `no-map-spread`.

## Issue Board (Kanban)

### Background refresh

Use `fetchIssuesWithExistingPagination("mutation")` for background refreshes — this keeps the board visible (no flash) and only shows a subtle loading indicator. Never use `fetchIssues` with `"init-loader"` for background work; it clears the board entirely.

The `useAutoRefreshIssues` hook (`apps/web/core/hooks/use-auto-refresh-issues.tsx`) implements this pattern: polls every 30 s, accepts a `shouldSkip` predicate. Always guard with:

- `isDragging` — skip while user is dragging a card
- loader state — skip while `"init-loader"` or `"pagination"` is active
- `document.activeElement` — skip when the user is typing in an `input`, `textarea`, or `contenteditable` element (protects quick-add forms and any inline editing)

```tsx
useAutoRefreshIssues(refreshFn, () => {
  if (isDragging) return true;
  if (issues.getIssueLoader() === "init-loader" || issues.getIssueLoader() === "pagination") return true;
  const el = document.activeElement;
  if (el) {
    const tag = el.tagName.toLowerCase();
    if (tag === "input" || tag === "textarea" || el.getAttribute("contenteditable") === "true") return true;
  }
  return false;
});
```

### Peek / side-card independence

The `peekIssue` state lives in the `issueDetail` MobX store and is **independent of board data**. Re-fetching board issues does not close the side card — safe to refresh in the background without disrupting the user's open detail view.

### Store signatures

Each issue store type (project, cycle, module, view) has its own signature for `fetchIssuesWithExistingPagination`. Check the relevant store before wiring up a refresh callback; the `refreshIssues` prop on `BaseKanBanRoot` abstracts over these differences.

## Issue Detail Page (browse)

`useAutoRefreshIssues` can also be used on `IssueDetailsPage` (`apps/web/app/(all)/[workspaceSlug]/(projects)/browse/[workItem]/page.tsx`). Call `fetchIssueWithIdentifier` directly — **not through SWR** — so background polls don't re-trigger the full-page skeleton loader (SWR's `isLoading` is only true on the initial fetch; direct store calls bypass it).

```tsx
const refreshIssue = useCallback(
  () => fetchIssueWithIdentifier(workspaceSlug, projectIdentifier, sequence_id).then(() => {}),
  [fetchIssueWithIdentifier, workspaceSlug, projectIdentifier, sequence_id]
);

useAutoRefreshIssues(refreshIssue, () => {
  if (issueLoader) return true;
  const el = document.activeElement;
  if (el) {
    const tag = el.tagName.toLowerCase();
    if (tag === "input" || tag === "textarea" || el.getAttribute("contenteditable") === "true") return true;
  }
  return false;
});
```

## Pages (collaborative editor)

A page is **not** rendered from `description_html`. The editor connects to the live server (Hocuspocus/Yjs), which loads the page from the Yjs snapshot in `pages.description_binary` and only falls back to converting `description_html` when that snapshot is empty (`apps/live/src/extensions/database.ts`). The page **title** is part of the same snapshot (the `title` Yjs fragment).

So any server-side write to `description_html` or `name` that bypasses the editor must also rewrite the snapshot — and it must rewrite it **inside the existing Yjs document**, through the live server: `POST {LIVE_URL}/document/<page_id>/content` (`apps/api/plane/utils/live_server.py` → `apps/live/src/controllers/document-content.controller.ts`). It takes the stored snapshot plus the new HTML/title and returns the snapshot to persist.

Do **not** build a fresh snapshot out of the HTML (e.g. the live server's `/convert-document/` endpoint, or clearing `description_binary` and letting the live server rebuild it). Two things break:

1. Yjs sync is a **merge**, never a replace. The browser caches the document in IndexedDB (`packages/editor/src/core/hooks/use-yjs-setup.ts`), so a client that opened the page before merges the old and the new document: the body shows both versions and the title becomes `"OldTitleNewTitle"`. It then pushes that merged state back to the server.
2. While any editor has the page open, the live server holds the document in memory; on its next store (debounced, and again on unload) it writes that state back over `description_binary` **and** `description_html`, silently reverting the change for good. When the live server that gets the request holds the document, it applies the rewrite to that in-memory copy — its snapshot is the only up-to-date one — and publishes the resulting Yjs update on the Redis admin channel, so every other server holding the document applies it and open editors update immediately.

There is **no fallback**. Clearing `description_binary`/`description_json` and letting the live server rebuild produces exactly the unrelated document above, so the API refuses the update with `503` and leaves the page alone when the live server is unreachable or unconfigured (`LiveServerUnavailable`). A page whose `description_binary` is still empty is updated without the live server — no client can be holding a snapshot of it.

Hand over only the fields that changed: the live server rewrites just the fragments it is given, so a body update does not revert a rename made in the editor, and vice versa.

`LIVE_BASE_URL` must be set **on the API container**, not just on the live one — it was missing from the deployment compose files, which is why the API silently took the destructive fallback in production.

## Running API tests

`apps/api` needs Python 3.12 (the code uses `X | Y` type syntax); the container's default `python3` is 3.9. Postgres/Redis come from the running `plane-test-*` containers:

```bash
cd apps/api && DATABASE_URL="postgresql://plane:plane@<plane-test-db ip>:5432/plane" \
  REDIS_URL="redis://<plane-test-redis ip>:6379/" \
  AMQP_URL="amqp://plane:plane@<plane-test-mq ip>:5672/plane" \
  WEB_URL="http://localhost:3000" SECRET_KEY="test" \
  /root/.venvs/plane312/bin/python -m pytest plane/tests/contract/api/test_pages.py
```

`AMQP_URL` and `WEB_URL` are **not optional**: any endpoint that fires `issue_activity.delay(...)`
returns a `500` without them (kombu cannot reach the broker, `base_host()` raises
`ImproperlyConfigured`), and the failure looks nothing like the missing setting. The broker
credentials are `plane:plane` on vhost `plane`, not the rabbitmq `guest` defaults — read them off
the container with `docker inspect plane-test-mq-1` rather than guessing.

Neither `ruff` nor a 3.12 `python` is on `PATH`; both live in `/root/.venvs/plane312/bin/`.

`pytest` **reuses** the test database, so a migration you just wrote is silently not applied and
every test touching the new column fails on `column … does not exist`. Pass `--create-db` after
adding a migration.

`Model.objects.create(created_by=user)` does **not** set `created_by`: `BaseModel.save()`
overwrites it from the thread-local request user, which is unset in a test, so the row lands with
`created_by=None`. Build the instance and call `instance.save(created_by_id=user.id)` instead —
this bites on any model whose endpoint scopes by author (drafts, for one).

`apps/api/run_tests.sh` is broken (it execs a `tests/run_tests.sh` that does not exist) — call `pytest` directly. CI lints with `ruff check`.

A handful of tests (`test_authentication.py` magic-link, `test_cycles.py`, `test_api_token.py`,
`test_url.py`, `test_copy_s3_objects.py` — 18 in all) fail on an untouched tree. Confirm a failure
is yours by re-running it with your changes stashed before chasing it.

## Work item filters (`plane/utils/filters/`)

Two things about `ComplexFilterBackend` + `IssueFilterSet` are easy to get wrong:

- **The allowlist is `filterset_class.base_filters`**, a class attribute built at import time. A
  filter declared per request (as the `property_<uuid>__<lookup>` custom property filters are, in
  `IssueFilterSet.__init__`) is invisible to it, so it also needs the
  `BaseFilterSet.is_dynamic_filter_name` hook — declaring the filter alone gets a
  `Filtering on field '…' is not allowed`.
- **A filter added after `super().__init__()` has no `parent`.** `FilterSet.__init__` wires
  `.parent`/`.model` onto the filters it already knew about, and a filter resolves its `method=`
  through its parent, so anything added afterwards must set both by hand or the request 500s with
  `must have a parent FilterSet to find '.filter_x()'`.

Multiple conditions in **one** `.filter()` call against a multi-valued relation must be satisfied by
the **same** related row. `build_combined_q` ANDs every leaf into a single `Q`, so a filter over a
one-to-many table (property values, and anything like it) has to be a `Q(pk__in=<subquery>)` per
condition — a plain join silently matches nothing as soon as there are two conditions.

## Frontend tests

There are none. `apps/web` has no test runner configured — its `package.json` scripts are only `dev`/`build`/`preview`/`start`/`clean` plus the `check:*`/`fix:*` gates. A web-only change is verified with `check:lint`, `check:format` and `check:types`; do not go looking for a Jest/Vitest setup to extend.

## GitHub / PR workflow

- **PR base branch is `master`** — always open PRs against `master`, not `preview` or `dev`.

## State Management

- MobX stores live in `packages/shared-state` and `apps/web/core/store`
- Use `observer()` from `mobx-react-lite` for reactive components
- Stores are injected via React context; access them with the typed `use*Store` hooks

## Code Style

- **Imports**: `workspace:*` for internal packages, `catalog:` for external deps
- **TypeScript**: Strict mode; all files must be typed
- **Formatting**: oxfmt — run `pnpm fix:format`
- **Linting**: OxLint with shared `.oxlintrc.json`. `pnpm check:lint` reports warnings but still
  exits `0`, while the husky pre-commit hook runs `oxlint --fix --deny-warnings` over the staged
  files — so a warning that already existed in a file you touched blocks the commit even though
  the repo-wide gate passed. Run `npx oxlint --deny-warnings <changed paths>` before committing.
- **Naming**: camelCase for variables/functions, PascalCase for components/types
- **Components**: Build in `@plane/ui` with Storybook for isolated development
