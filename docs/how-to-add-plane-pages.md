# How to add Plane pages for a project

Plane's Pages feature (separate from Issues/tasks) is where per-project
documentation lives — architecture notes, module writeups, runbooks. This is
the convention every project uses, via `plane.sh`'s `*-page` commands. It is
the same for every stack (JS, PHP, Python, infra) — page content is just rich
text, so there is nothing language-specific about how pages are organized.

## Constraint: the Plane API has no `parent_id` for pages

Plane's page-hierarchy (the tree you see in the sidebar, where sub-pages nest
under a parent) is UI-only — creating a page via the API always creates it as
a flat, top-level page in the project. There is no `parent`/`parent_id` field
to set at creation time. This means:

- The naming convention below (not real nesting) is what makes pages
  discoverable via `search-pages`, regardless of whether anyone has nested
  them in the UI.
- If you want the visual tree to match, drag a page under its parent in the
  Plane UI once after creating it. This is a manual, one-time step — `plane.sh`
  cannot do it for you.

## Finding your root page(s): `main-page`

`docs/plane.sh main-page [page_name] [env_key]` is the single command for
this — it finds or creates a root page and always returns it:

- If `env_key` (default `PLANE_MAIN_DOC_PAGE_ID`) is already set in `.env`,
  it does a direct `get-page` by id — no search involved.
- Otherwise it searches for a page named exactly `page_name`. If found, that
  page is used; if not, it is created. Either way the resolved id is
  written to `env_key` in `.env` (untracked, alongside the other real
  `PLANE_*` deploy values for this project) so every later call takes the
  direct-lookup path above.

The response always includes `"just_created": true` or `false`. On
`true`, the page was just made and is empty (`<p></p>`) — populate it with
landing-page content per step 1 below, in the same run. On `false`, it
already existed — do not overwrite its `description_html` wholesale;
follow the get-page/edit-page file round trip in the Commands reference
if you need to change it.

Most projects have exactly one root (the default `PLANE_MAIN_DOC_PAGE_ID`
key). A project can instead keep two fully independent hierarchies — see
"Dual-hierarchy option" below — by calling `main-page` twice with two
different `page_name`/`env_key` pairs.

## Convention

### 1. One main page per project

There is exactly one top-level page named after the project, matching
`PROJECT_NAME` from that project's config (e.g. `jobscanner-python`,
`k8s-stack`) — `main-page "$PROJECT_NAME"` finds or creates it (see above).
This is the landing page, and it **must** contain a link to every other
page that exists for the project — see step 4, which is not optional. The
first time it is created (`just_created: true`), give it real landing-page
content instead of leaving it blank:

```bash
docs/plane.sh edit-page "$PLANE_MAIN_DOC_PAGE_ID" "" "Landing page for jobscanner-python docs. See sub-pages below."
```

### 2. Dev docs / User docs split, directly under the main page

Every project gets exactly two second-level pages, named with a
`ProjectName: ` prefix:

```bash
docs/plane.sh create-page "jobscanner-python: Developer Docs" "..."
docs/plane.sh create-page "jobscanner-python: User Docs" "..."
```

- **Developer Docs** — architecture, module internals, how things fit
  together, anything aimed at whoever is implementing tasks (including the
  ralph agent itself).
- **User Docs** — how to use the running system as an end user/operator, with
  no assumption of familiarity with the code.

### 3. Sub-pages for modules/themes, nested under Dev or User docs

Use the same `ProjectName: ` prefix for every further sub-page, with a
descriptive name after it:

```bash
docs/plane.sh create-page "jobscanner-python: Scraper Module" "..."
docs/plane.sh create-page "jobscanner-python: Auth Theme" "..."
```

Put module/internals pages under Developer Docs and workflow/feature pages
under User Docs. If a topic genuinely serves both audiences, pick the one a
newcomer would look for first — do not duplicate the page.

After creating any of these, drag it under its intended parent in the Plane
UI (once) so the sidebar tree matches the naming convention.

### 4. Link every new page from the main page

The main page should hold a list of links to every other page for the
project, including module/theme sub-pages. Get the new page's URL with
`page-url`, then use `get-page`/`edit-page` (see Commands reference) to add
it to the main page's `description_html` — check the existing content first
so a re-run does not add a duplicate link.

## Dual-hierarchy option

The convention above (steps 1-4) is the default, `DOCS_SNIPPET=single-main`
layout: one main page with a Developer Docs / User Docs split beneath it.
Set `DOCS_SNIPPET=dual-hierarchy` in a project's config instead to keep dev
docs and user/support docs as two fully independent hierarchies — no shared
main page, each with its own root and its own set of links:

```bash
docs/plane.sh main-page "jobscanner-python: Developer Docs" PLANE_DEV_DOCS_PAGE_ID
docs/plane.sh main-page "jobscanner-python: User Docs" PLANE_USER_DOCS_PAGE_ID
```

Everything else — the `ProjectName: ` sub-page naming convention, dragging
pages under their parent in the UI, linking each new sub-page from its
root's `description_html` — works the same as steps 3-4 above, just against
whichever root matches the new page's audience instead of a shared main
page. See `snippets/docs-section/dual-hierarchy.md` for the exact prompt
wording used when this is enabled.

## Commands reference

```bash
docs/plane.sh create-page <name> [desc_html|@file]      # @file reads desc from a file
docs/plane.sh main-page [page_name] [env_key]           # find/create + get a root page; just_created: true/false — see above
docs/plane.sh page-url <page_id>                        # web-app URL, for linking a page from another page
docs/plane.sh get-page <page_id> [out_file]             # with out_file, writes description_html to it instead of printing page JSON
docs/plane.sh edit-page <page_id> [name] [desc_html|@file]  # either arg "" to leave untouched; @file reads desc from a file
docs/plane.sh rename-page <page_id> <name>              # rename only, description untouched
docs/plane.sh search-pages <query>                      # e.g. search-pages "jobscanner-python"
docs/plane.sh archive-page <page_id>                    # required before remove-page
docs/plane.sh remove-page <page_id>
```

To edit an existing page — including appending a link, as in step 4 above —
`get-page <id> page.html` to save the current description_html to a file,
edit (or append to) that file, then push the whole thing back with
`edit-page <id> "" @page.html`. There is no standalone append command;
this file round trip covers both appends and larger rewrites and avoids
juggling escaped HTML on the command line.

Before creating a new page, `main-page`/`search-pages "ProjectName"` first —
if the main page or the Dev/User Docs pages already exist, edit them
(`edit-page`) instead of creating duplicates.
