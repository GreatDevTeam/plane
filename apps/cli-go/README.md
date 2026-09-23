# plane-cli

A terminal (TUI) client for Plane, for viewing and updating work items from a keyboard.
Built and tested for Ubuntu.

Scope, for now: work items only — projects act as boards, work items are grouped into
columns by state. You can read and update an item (state, priority, description), and read,
write and edit its comments. Creating/deleting work items and other Plane features are not
covered yet.

## Build

Requires Go 1.23+.

```bash
cd apps/cli-go
go build -o plane-cli ./cmd/plane-cli
./plane-cli
```

## Sign in

On first run you'll be asked for:

1. **Server URL** — e.g. `https://app.plane.so` or your self-hosted instance's URL.
2. **API token** or **email/password**.
   - An API token is created in Plane under _Workspace settings -> API tokens_ and is the
     more direct option. Since a bare token has no session, plane-cli cannot look up which
     workspaces it can access, so it then asks for the **workspace slug** directly — the
     `<slug>` in `https://app.plane.so/<slug>/...`.
   - With email/password, plane-cli signs in the same way the web app does and mints a
     permanent API token on your behalf for future runs, so you only type your password
     once. Signing in this way, it can also list the workspaces you belong to (the session
     from sign-in briefly reaches the same endpoint the web app uses to fill its workspace
     switcher): if you belong to exactly one, it is selected automatically; with more than
     one, you get a picker instead of typing the slug. `w` on the boards list or the board
     screen reopens that picker.

The server URL, token, and workspace slug are saved to `~/.config/plane-cli/config.json`
(mode `0600`) so subsequent runs skip straight to the board.

## Shortcuts

| Screen           | Keys                                                             |
| ---------------- | ---------------------------------------------------------------- |
| Any screen       | `?` toggle help, `ctrl+c` quit                                   |
| Text field       | `enter` confirm, `esc` back                                      |
| Workspace picker | `j`/`k` move, `enter` select, `esc` type slug instead            |
| Boards list      | `j`/`k` move, `enter` open board, `w` switch workspace, `q` quit |
| Board            | `h`/`l` switch column, `j`/`k` move card, `enter` open item      |
|                  | `s` change state, `y` change priority                            |
|                  | `a` filter by assignee, `L` filter by label                      |
|                  | `o` card order, `x` hide/show the focused column                 |
|                  | `n` new work item                                                |
|                  | `r` refresh now, `p` switch board (project), `q` quit            |
| Item detail      | `s` change state, `y` change priority, `d` edit description      |
|                  | `g` go to parent, `S` jump to a sub-task                         |
|                  | `tab` switch pane, `j`/`k` scroll / select comment               |
|                  | `c` add comment, `e` edit own comment                            |
|                  | `esc`/`backspace` back, `q` quit                                 |
| Editor           | `ctrl+s` save, `esc` cancel                                      |
| Picker           | `j`/`k` move, `enter` apply, `esc` cancel                        |

## Board behaviour

- **Column order** matches the web app: states are ordered by their group (backlog,
  unstarted, started, completed, cancelled) and then by their sequence inside that group.
- **Card order** inside a column is chosen with `o`: the API's own order (default), priority,
  created date (newest or oldest first), last updated, name, or work item number. The choice
  is saved to the config file.
- **Parent and sub-tasks** show on every card's bottom line: `↑2113` is the work item this
  card is a sub-task of, `↳3` is how many sub-tasks hang off it. The detail screen lists the
  parent and the sub-tasks in full, each with its own state and priority; `g` opens the
  parent and `S` opens a picker to jump to a sub-task. Plane's REST API only sends a work
  item's `parent`, never its children, so sub-tasks are worked out from the project's own
  work items — which is also why they are always within one project.
- **Hiding a column** with `x` collapses it to a narrow placeholder — its name stays on the
  board, stacked vertically, so you can bring it back with `x` — and hands its width to the
  columns that are still expanded. Which columns are collapsed is saved per project.
- **The board refreshes itself every 30 seconds.** The refresh happens in the background and
  the new board is swapped in whole, so it never blinks and never loses your place: the
  focused column, each column's cursor and the active filters all survive it. A refresh is
  skipped while you are typing in an editor, while a picker is open, and while the board is
  still loading. `r` runs the same refresh immediately.
- **Opening a card** shows the board's cached copy straight away and re-fetches the item and
  its comments in the background; the footer says which of the two is still in flight.
- **Editing a description** (`d` on the detail screen) works on plain text: the current
  description is flattened to text to edit, and saved back as one paragraph per line — the
  same `description_html` field, and the same payload, the web app's editor sends. Rich
  formatting written in the browser (lists, bold, links) is flattened by an edit from here.

On a narrow terminal (not wide enough to fit every column at a readable width), the board
shows only the focused column, full width, instead of squeezing all of them in; `h`/`l` still
switches which one is focused. A column taller than the terminal clips to the visible rows
around the cursor (marked with a trailing `*` on its header) so the header never scrolls
off-screen.

## Notes

- Only the public REST API (`/api/v1/`, `X-Api-Key` auth) is used once signed in; it is the
  officially supported integration surface, unlike the web app's session/CSRF endpoints
  (which are only used transiently during an email/password sign-in, to mint the token and,
  opportunistically, to list workspaces — see _Sign in_ above).
- There is still no "list my workspaces" endpoint on the public API itself, so a bare API
  token (no session) falls back to asking for the workspace slug directly.
