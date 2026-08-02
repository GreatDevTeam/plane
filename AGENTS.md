# Agent Development Guide

## Commands

- `pnpm dev` - Start all dev servers (web:3000, admin:3001)
- `pnpm build` - Build all packages and apps
- `pnpm check` - Run all checks (format, lint, types)
- `pnpm check:lint` - OxLint across all packages
- `pnpm check:types` - TypeScript type checking (requires turbo + installed node_modules; not available in the dev container)
- `pnpm fix` - Auto-fix format and lint issues
- `pnpm turbo run <command> --filter=<package>` - Target specific package/app
- `pnpm --filter=@plane/ui storybook` - Start Storybook on port 6006

## Code Style

- **Imports**: Use `workspace:*` for internal packages, `catalog:` for external deps
- **TypeScript**: Strict mode enabled, all files must be typed
- **Formatting**: oxfmt, run `pnpm fix:format`
- **Linting**: OxLint with shared `.oxlintrc.json` config
- **Naming**: camelCase for variables/functions, PascalCase for components/types
- **Error Handling**: Use try-catch with proper error types, log errors appropriately
- **State Management**: MobX stores in `packages/shared-state`, reactive patterns
- **Testing**: All features require unit tests, use existing test framework per package
- **Components**: Build in `@plane/ui` with Storybook for isolated development

## Issue Board Patterns

### Background refresh (Kanban)

Use `fetchIssuesWithExistingPagination("mutation")` for background refreshes — this keeps the board visible (no flash) and shows only a subtle loading indicator. Do **not** use `fetchIssues` with `"init-loader"` for background work; it clears the board.

The `useAutoRefreshIssues` hook (`apps/web/core/hooks/use-auto-refresh-issues.tsx`) encapsulates this pattern with a 30 s interval. Always guard its `shouldSkip` predicate with:

1. `isDragging` — skip while a card is being dragged
2. Loader state — skip while `"init-loader"` or `"pagination"` is active
3. `document.activeElement` — skip when the user is typing in an `input`, `textarea`, or `contenteditable` element (protects quick-add forms and inline editors)

### Peek / side-card independence

The `peekIssue` state lives in the `issueDetail` MobX store and is **independent of board data**. Re-fetching board issues does not close the peek panel — safe to refresh in the background without disrupting an open detail view.

### Store refresh signatures

Each store type (project, cycle, module, view) has its own signature for `fetchIssuesWithExistingPagination`. The `refreshIssues` prop on `BaseKanBanRoot` abstracts over these differences — implement it per store type in `use-issues-actions.tsx`.

## Pages (collaborative editor)

A page is **not** rendered from `description_html`. The editor connects to the live server (Hocuspocus/Yjs), which loads the page from the Yjs snapshot in `pages.description_binary` and only converts `description_html` when that snapshot is empty (`apps/live/src/extensions/database.ts`). The page **title** is part of the same snapshot, as the `title` Yjs fragment.

A server-side write to `description_html` or `name` that bypasses the editor therefore has to rewrite the snapshot — and it has to rewrite it **inside the existing Yjs document**, via `POST {LIVE_URL}/document/<page_id>/content` (`apps/api/plane/utils/live_server.py` → `apps/live/src/controllers/document-content.controller.ts`). Building a fresh snapshot out of the HTML instead breaks in two ways:

1. Yjs sync is a merge, never a replace. The browser caches the document in IndexedDB (`packages/editor/src/core/hooks/use-yjs-setup.ts`), so a client that had opened the page before merges the old and the new document and ends up showing both versions — and pushes that merged state back to the server.
2. While any editor has the page open, the live server holds the document in memory and stores that state back over `description_binary` **and** `description_html`, silently reverting the change. The live endpoint pushes the same Yjs update to every server holding the document (Redis admin channel), so open editors pick the change up instead.

Clearing `description_binary`/`description_json` is only the fallback for when the live server is unreachable: the snapshot is then rebuilt from `description_html` plus the page name the next time the page is opened.
