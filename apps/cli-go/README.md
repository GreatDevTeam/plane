# plane-cli

A terminal (TUI) client for Plane, for viewing and updating work items from a keyboard.
Built and tested for Ubuntu.

Scope, for now: work items only — projects act as boards, work items are grouped into
columns by state. Creating/deleting work items, comments, and other Plane features are not
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
|                  | `r` refresh, `p` switch board (project), `q` quit                |
| Item detail      | `s` change state, `y` change priority, `esc`/`backspace` back    |
| Picker           | `j`/`k` move, `enter` apply, `esc` cancel                        |

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
