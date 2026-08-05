# Custom fields on work items — investigation

_Plane task #890. Investigates what it takes to have user-defined custom fields
("custom properties") on work items, visible in filters, board, work item card,
detail view and API._

## Verdict

Custom fields are an **Enterprise-edition feature of upstream Plane**. This repo is the
community edition (`ce`), and upstream ships it as a set of **no-op stubs**: the extension
points exist and are already wired into every surface that matters, but every one of them
returns `null`/`<></>`/`{}`.

That is good news — we do **not** need to touch the core rendering code of the board, the
card, the spreadsheet, the peek view or the filter bar. Implementing custom fields means:

1. adding the missing **database models + API** (nothing exists server side), and
2. **filling in the CE stubs** listed below (plus three small type/constant lists that are
   closed unions today).

## How the CE / EE seam works

`apps/web/tsconfig.json` maps `@/plane-web/*` → `./ce/*`. Everything the paid edition
overrides is imported through `@/plane-web/...`, so a file under `apps/web/ce/` is the
override point. Core code already calls these stubs at the correct place in the tree.

| Surface                                      | Core call site                                                                                                                             | Stub to implement                                                                                                                                                                                                             |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Board / list / calendar card                 | `apps/web/core/components/issues/issue-layouts/properties/all-properties.tsx:475`                                                          | `apps/web/ce/components/issues/issue-layouts/additional-properties.tsx`                                                                                                                                                       |
| Spreadsheet columns                          | `apps/web/core/components/issues/issue-layouts/spreadsheet/issue-column.tsx:32` (`SPREADSHEET_COLUMNS`)                                    | `apps/web/ce/components/issues/issue-layouts/utils.tsx`                                                                                                                                                                       |
| Create / update work item modal              | `apps/web/core/components/issues/issue-modal/form.tsx:482`                                                                                 | `apps/web/ce/components/issues/issue-modal/modal-additional-properties.tsx`                                                                                                                                                   |
| Modal form state (values, validation, save)  | `apps/web/core/components/issues/issue-modal/context/issue-modal-context.tsx`                                                              | `apps/web/ce/components/issues/issue-modal/provider.tsx`                                                                                                                                                                      |
| Detail sidebar                               | `apps/web/core/components/issues/issue-detail/sidebar.tsx:269`                                                                             | `apps/web/ce/components/issues/issue-details/additional-properties.tsx`                                                                                                                                                       |
| Peek overview sidebar                        | `apps/web/core/components/issues/peek-overview/properties.tsx:263`                                                                         | same as above                                                                                                                                                                                                                 |
| Property prefetch on peek / browse           | `apps/web/core/components/issues/peek-overview/root.tsx:53`, `apps/web/app/(all)/[workspaceSlug]/(projects)/browse/[workItem]/page.tsx:68` | `apps/web/ce/hooks/use-issue-properties.tsx`                                                                                                                                                                                  |
| Filter bar (which filters exist)             | `apps/web/core/components/work-item-filters/filters-hoc/base.tsx:72`                                                                       | `apps/web/ce/hooks/work-item-filters/use-work-item-filters-config.tsx`                                                                                                                                                        |
| Filter value editor for non-core types       | `apps/web/core/components/rich-filters/filter-value-input/root.tsx:84`                                                                     | `apps/web/ce/components/rich-filters/filter-value-input/root.tsx`                                                                                                                                                             |
| Activity feed entries for property changes   | `apps/web/core/components/issues/issue-detail/issue-activity/activity-comment-root.tsx:86`                                                 | `apps/web/ce/components/issues/issue-details/issue-properties-activity/root.tsx`                                                                                                                                              |
| Work item type chip / switcher / type filter | —                                                                                                                                          | `apps/web/ce/components/issues/issue-details/issue-type-switcher.tsx`, `.../issue-identifier.tsx`, `apps/web/ce/components/issues/filters/issue-types.tsx`, `apps/web/ce/components/issues/issue-modal/issue-type-select.tsx` |

The modal context is the clearest evidence that the contract is already designed for this —
`apps/web/ce/components/issues/issue-modal/provider.tsx` supplies `issuePropertyValues`,
`setIssuePropertyValues`, `issuePropertyValueErrors`, `getActiveAdditionalPropertiesLength`,
`handlePropertyValuesValidation` and `handleCreateUpdatePropertyValues` as empty
implementations. Core forms already call all of them.

## What exists server side

Only **work item types**, and only partially:

- `apps/api/plane/db/models/issue_type.py` — `IssueType` (`issue_types` table, incl.
  `is_epic`, `is_default`, `level`, `logo_props`) and `ProjectIssueType`
  (`project_issue_types`, the per-project enablement join).
- `Issue.type` FK (`apps/api/plane/db/models/issue.py:163`) and the same on
  `DraftIssue` (`apps/api/plane/db/models/draft.py:71`). Tables and columns are created by
  migrations `0070_*` and `0074_*`, so an existing deployment already has them.
- The public API v1 serializer accepts and returns `type_id`
  (`apps/api/plane/api/serializers/issue.py:66`) and falls back to the project's default
  type on create (`:159-166`).

What is **missing** server side:

- No CRUD endpoints for `IssueType` / `ProjectIssueType` anywhere (`apps/api/plane/app/urls/`
  has no issue-type route; `IssueType` is referenced only by the v1 issue serializer).
- No default type is ever seeded — `apps/api/plane/bgtasks/workspace_seed_task.py` only sets
  the `issue_type` _display property_ flag, it does not create an `IssueType` row. So
  `type_id` is `null` for every work item today.
- **No property models at all** — there is no `IssueProperty`, `IssuePropertyOption` or
  `IssuePropertyValue`. (`IssueProperty` in migration `0053` is the old name of
  `IssueUserProperty`/`ProjectUserProperty`, i.e. per-user view preferences — unrelated.)
- The web (`/api/v1/…app`) issue list payload does not even return `type_id`:
  `required_fields` in `apps/api/plane/app/views/issue/base.py:858-885` omits it.

## Closed lists that have to be opened up

These three are hard-coded unions today and are what makes "a filter/column per user-defined
field" impossible without a change:

1. `WORK_ITEM_FILTER_PROPERTY_KEYS` — `packages/types/src/view-props.ts:96-112`. A closed
   `as const` tuple; `TWorkItemFilterConditionKey` is `${property}__${operator}`. Custom
   fields need a templated key (e.g. `property_<uuid>`), so the type has to become
   `TWorkItemFilterProperty | \`property\_${string}\``.
2. `EXTENDED_FILTER_FIELD_TYPE` — `packages/types/src/rich-filters/field-types/extended.ts`
   is literally `{} as const` with `TExtendedFilterFieldConfigs = never`. This is the
   designated slot for the non-core filter editors (text, number, boolean, url, email,
   datetime, member) that custom fields need, and it pairs with the
   `AdditionalFilterValueInput` stub.
3. `IIssueDisplayProperties` / `ISSUE_DISPLAY_PROPERTIES_KEYS` —
   `packages/types/src/view-props.ts:161-177` and
   `packages/constants/src/issue/common.ts:142-159`. Card/spreadsheet visibility toggles are
   keyed by a fixed union, so per-field toggles need the same widening.

Server side the equivalent gate is the filter allowlist: `ComplexFilterBackend` rejects any
field not declared on the view's `filterset_class`
(`apps/api/plane/utils/filters/filter_backend.py:100-125`), and `IssueFilterSet`
(`apps/api/plane/utils/filters/filterset.py:124-170`) declares a fixed set. The backend
already anticipates this — the docstring at `filter_backend.py:222` says _"custom property
filters might need to be transformed from `customproperty_<id>**<lookup>`to`customproperty_value**<lookup>`"_ — but nothing implements it. The legacy filter path
(`apps/api/plane/utils/issue_filters.py:428-463`, the `ISSUE_FILTER` dispatch dict) is a
second allowlist that would need the same treatment if legacy filters stay in use.

## Proposed implementation

### 1. Data model (`apps/api/plane/db/models/issue_property.py`)

```
IssueProperty          workspace, issue_type (FK), name, display_name, description,
                       property_type ("TEXT"|"DECIMAL"|"OPTION"|"BOOLEAN"|"DATETIME"|"RELATION"|"URL"|"EMAIL"|"FILE"),
                       relation_type (null|"USER"|"ISSUE"), is_required, is_active,
                       is_multi, default_value (JSON), settings (JSON), sort_order,
                       logo_props, external_source, external_id
IssuePropertyOption    workspace, property (FK), name, sort_order, is_active, is_default,
                       description, logo_props, parent
IssuePropertyValue     workspace, project, issue (FK), property (FK),
                       value_text, value_decimal, value_boolean, value_datetime,
                       value_uuid (option / user / issue), external_source, external_id
IssuePropertyActivity  workspace, project, issue, property, old_value, new_value, action, comment
```

Properties hang off `IssueType`, matching the FK that already exists on `Issue`; a work item
gets the property set of its type. Values are one row per (issue, property, value) so
multi-value fields work, with typed columns instead of a single JSON blob so that filtering,
ordering and grouping stay index-friendly.

**Prerequisite:** work item types have to actually work first — an endpoint to create them,
a default type seeded per project (backfilling `Issue.type_id` for existing rows), and
`type_id` added to the list payload. Without that there is nothing to attach properties to.

### 2. API

- `/api/v1/workspaces/<slug>/issue-types/` + `/issue-types/<id>/issue-properties/` +
  `/issue-properties/<id>/options/` — CRUD (admin-only writes).
- `/api/v1/workspaces/<slug>/projects/<id>/issues/<issue_id>/issue-property-values/` —
  read/replace the values of one work item.
- A **bulk** values endpoint for a list of issue ids: board/list/spreadsheet responses are
  built with `.values(...)` (`apps/api/plane/app/views/issue/base.py:858`) and must stay
  cheap, so property values should be fetched in a second request per page of issues, the
  same way `useWorkItemProperties` is already positioned to do it.
- Filtering: add a `filter_custom_property` method to `IssueFilterSet` that accepts
  `property_<uuid>__<op>` keys and rewrites them onto `IssuePropertyValue` subqueries — the
  transformation the `filter_backend.py:222` docstring already describes.
- Activity: emit `IssuePropertyActivity` from the value endpoints so the detail feed has
  something to render.

#### What phase 1 shipped

All under `/api/` (the app API, not the public `/api/v1/` one), in
`apps/api/plane/app/urls/issue_property.py`:

| Endpoint                                                                   | Methods                       | Who                              |
| -------------------------------------------------------------------------- | ----------------------------- | -------------------------------- |
| `workspaces/<slug>/issue-types/<issue_type_id>/issue-properties/[<pk>/]`   | `GET` `POST` `PATCH` `DELETE` | read: any member, write: admin   |
| `workspaces/<slug>/issue-properties/<property_id>/options/[<pk>/]`         | `GET` `POST` `PATCH` `DELETE` | read: any member, write: admin   |
| `workspaces/<slug>/projects/<id>/issues/<issue_id>/issue-property-values/` | `GET` `POST`                  | read: any member, write: ≥member |
| `workspaces/<slug>/projects/<id>/issue-property-values/?issue_ids=a,b,…`   | `GET`                         | any member                       |

Values are exchanged as `{"<property_id>": [value, …]}` — always a list, even for a
single-valued property, and always present for every property of the work item's type so the
client can tell "not set" from "not loaded". The bulk endpoint nests that one level deeper
(`{"<issue_id>": {"<property_id>": […]}}`), gives each work item only the properties of its
own type, and is capped at 500 ids.

`POST`ing values replaces only the properties named in the payload (`{"property_values": {…}}`,
or the body itself), so a partial save from the sidebar does not wipe the rest of the form; an
empty list clears a property. Values are coerced into the typed column that matches
`property_type` and validated against `is_required`, `is_multi`, the option/member/work item
they point at, and the `settings` bounds (`min`, `max`, `max_length`) — a rejection comes back
as `{"<property_id>": "<message>"}` with a `400`. `property_type` is immutable once set, since
the stored values live in the column of the original type.

### 3. Frontend

- New MobX stores (`issue-types`, `issue-properties`, `issue-property-values`) registered on
  `apps/web/ce/store/root.store.ts`, plus services under `packages/services/src/issue/`.
- Widen the three closed unions above.
- Fill in the stubs from the table, in this order — each is independently shippable:
  detail sidebar → create/update modal (+ provider) → card/board → spreadsheet column →
  filters → activity feed.
- Settings UI for defining types and their fields (project settings → _Work item types_).

### 4. Phasing

| Phase | Scope                                                                                                         |
| ----- | ------------------------------------------------------------------------------------------------------------- |
| 0     | Work item types: CRUD API, default type seeding + backfill, `type_id` in list payload, type chip/select in UI |
| 1     | Property + option + value models, migrations, admin CRUD API                                                  |
| 2     | Read/write values on the work item detail sidebar and peek view                                               |
| 3     | Create/update modal support (validation + save via the existing context contract)                             |
| 4     | Display on cards and as spreadsheet columns (incl. display-property toggles)                                  |
| 5     | Filtering (backend filterset + `EXTENDED_FILTER_FIELD_TYPE` + filter config)                                  |
| 6     | Activity feed, import/export, webhooks                                                                        |

Phase 0 is a hard prerequisite for everything else. Phases 2–6 all depend on phase 1 and can
otherwise proceed in parallel.

## Risks / open questions

- **Licensing.** Upstream ships this as a paid feature. The stubs are AGPL-3.0 and re-filling
  them is fine, but we cannot copy EE source. Everything above is a from-scratch design that
  matches the existing stub signatures.
- **Merge conflicts with upstream.** Widening `IIssueDisplayProperties`,
  `WORK_ITEM_FILTER_PROPERTY_KEYS` and `IssueFilterSet` touches files upstream changes often.
  Keeping the additions additive (template literal unions, a single extra filter method)
  limits the blast radius.
- **Scope of "per type" vs "per project".** Properties are modelled per work item type here.
  If we want project-wide fields without types, phase 0 still applies — every project just
  gets one default type.
- **Filter performance** on `IssuePropertyValue` subqueries at workspace scope needs indexes
  on `(property_id, value_*)` and `(issue_id, property_id)`; worth a query plan check before
  phase 5 ships.
