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
     more direct option.
   - With email/password, plane-cli signs in the same way the web app does and mints a
     permanent API token on your behalf for future runs, so you only type your password once.
3. **Workspace slug** — the `<slug>` in `https://app.plane.so/<slug>/...`.

The server URL, token, and workspace slug are saved to `~/.config/plane-cli/config.json`
(mode `0600`) so subsequent runs skip straight to the board.

## Shortcuts

| Screen      | Keys                                                             |
| ----------- | ---------------------------------------------------------------- |
| Any screen  | `?` toggle help, `ctrl+c` quit                                   |
| Text field  | `enter` confirm, `esc` back                                      |
| Boards list | `j`/`k` move, `enter` open board, `w` switch workspace, `q` quit |
| Board       | `h`/`l` switch column, `j`/`k` move card, `enter` open item      |
|             | `s` change state, `y` change priority, `r` refresh               |
|             | `p` switch board (project), `q` quit                             |
| Item detail | `s` change state, `y` change priority, `esc`/`backspace` back    |
| Picker      | `j`/`k` move, `enter` apply, `esc` cancel                        |

## Notes

- Only the public REST API (`/api/v1/`, `X-Api-Key` auth) is used once signed in; it is the
  officially supported integration surface, unlike the web app's session/CSRF endpoints
  (which are only used transiently during an email/password sign-in).
- There is no "list my workspaces" endpoint on that API, which is why the workspace slug is
  asked for directly rather than offered as a list.
