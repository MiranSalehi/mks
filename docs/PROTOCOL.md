# API protocol (v0)

The contract between the UI and any backend. Every adapter (Goravel, and later Laravel or Rust) must implement exactly this.
The TypeScript types for this contract live in `src/protocol.ts`.

> Status: v0, finalized at the end of phase 0. The sections marked ✅ are implemented and tested in this repository; the rest are designed and are implemented in the phase named.

## General rules

- Prefix for all paths: `{panelPath}/api` (default: `/admin/api`)
- Format: JSON, `Content-Type: application/json` (except uploads)
- Authentication: session cookie (`HttpOnly`, `Secure`, `SameSite=Lax`)
- CSRF: every non-GET request must carry an `X-CSRF-Token` header
- Field names: `snake_case`, the same as the model's column/field name
- Record id: the type the backend has (`42` or `"uuid"`). In the UI its type is `string | number` and in URLs it is always a string.
- **Dates and times (output):** always ISO 8601 in UTC, like `"2026-11-19T09:08:18Z"`. An empty value is `null`.
- **Dates and times (input):** ISO 8601 with or without a time zone (`2026-11-19T12:30:00+03:30`, `2026-11-19`). Without a time zone it is taken as UTC. `null` or `""` clears the value.
- **Relations:** a relation requested with `include` always has a key: an empty list is `[]` and a missing record is `null`. A relation that was not requested does not appear in the response.

## Error format

| Code | Meaning | Body |
|---|---|---|
| 400 | invalid request (a parameter outside the allowlist, a malformed body) | `{ "message": "..." }` |
| 401 | not signed in | `{ "message": "..." }` |
| 403 | not allowed | `{ "message": "..." }` |
| 404 | not found | `{ "message": "..." }` |
| 409 | conflict with a database uniqueness constraint (two concurrent requests with the same unique value) | `{ "message": "..." }` |
| 419 | invalid or expired CSRF | `{ "message": "..." }` |
| 422 | validation error | `{ "message": "...", "errors": { "title": ["is required"], "tags.0.name": ["..."] } }` |
| 429 | too many requests | `{ "message": "...", "retry_after": 30 }` |

`errors` keys use dots to point at nested fields (for the Repeater and Builder).

**Message language:** messages are in the `panel.locale` language (currently `fa` and `en`). A field's error message **does not repeat the field name** ("This field is required."), because the UI shows it under that field and field labels exist only in the frontend definitions. Messages meant for developers (such as a 400 for a disallowed parameter) stay in English.

---

## ✅ Authentication

| Method | Path | Needs login | Notes |
|---|---|---|---|
| GET | `/auth/csrf` | no | `{ "token": "..." }` |
| POST | `/auth/login` | no | `{ email, password }` ← `{ "user": {...}, "csrf_token": "..." }`; bad credentials: 422 on `email`; more than 5 attempts per minute: 429 |
| GET | `/auth/me` | no | `{ "user": {...} }` or 401 |
| POST | `/auth/logout` | yes | `{ "csrf_token": "..." }` |

- Every other path returns **401** without a login.
- The session is kept in a cookie (`HttpOnly`, `SameSite=Lax`).
- **CSRF:** every non-GET request must carry the `X-CSRF-TOKEN` header, otherwise **419**. The server returns the current token in that header on every response. Login and logout change the token and return the new one in the body. On a 419 the client fetches a fresh token once and retries.
- `user` contains `id`, `name` and `email`.

## ✅ Meta

### `GET /meta`

What the server allows, for the current user. Labels and layout live in the frontend TSX definitions, not here.

```json
{
  "protocol": 0,
  "panel": { "path": "/admin", "brand": "Admin panel", "locale": "fa", "direction": "rtl", "notifications": { "poll": 15 } },
  "user": { "id": 1, "name": "...", "email": "..." },
  "order": ["posts", "categories"],
  "widgets": ["overview", "published_per_month"],
  "pages": ["authors"],
  "search": true,
  "extra": { "navigation": { "items": {}, "groups": {} } },
  "resources": {
    "posts": {
      "key": "id",
      "fields": {
        "title":        { "type": "string",   "nullable": false },
        "category_id":  { "type": "integer",  "nullable": false },
        "published_at": { "type": "datetime", "nullable": true }
      },
      "relations": ["author", "category", "tags"],
      "capabilities": {
        "searchable": ["title", "excerpt"],
        "sortable":   ["id", "title", "created_at"],
        "filterable": { "status": ["eq", "in"], "published_at": ["gte", "lt", "null"] },
        "filters":    ["mine", "tags"],
        "summaries":  { "views": ["sum", "avg", "max"] },
        "reorderable": "",
        "fillable":   ["title", "slug", "category_id"]
      },
      "abilities": { "create": true, "delete": true, "header_actions": { "import": false } }
    }
  }
}
```

- Field types: `string`, `integer`, `number`, `boolean`, `datetime`. They are read from the model.
- `protocol` is the server's protocol version; the UI warns when it differs from its own.
- The UI shows only resources that have a TSX definition and also appear in `resources`. In development mode, differences (a `sortable()` column the server has not allowed, say) are warned about in the console.
- Only resources the user may view (`ViewAny`) appear. `abilities` says what the user can do (see the "Permissions" section).
- `page_badges` (optional) are the counts next to custom pages: `{ "system_health": { "count": 2, "color": "danger" } }`; zero means no badge.
- `extra` (optional) is plugin-wide server data, keyed by the name of each value (in Goravel: `Panel.MetaValue`). The UI applies two keys itself: `navigation` (sidebar layout) and `translations` (custom interface texts, `{ locale: { key: text } }`). An invalid value is ignored.
- With a global gate, a resource, widget or page whose permission you lack does not appear in meta at all, and `abilities` is the combined result of policy and gate.
- The TypeScript types are generated directly from the Go models (`./artisan mks:types`) and follow these same rules.

---

## Records

### ✅ `GET /resources/{resource}/records`

| Parameter | Example | Notes |
|---|---|---|
| `page` | `1` | |
| `per_page` | `25` | the server sets the cap (100, say) |
| `sort` | `-created_at,title` | `-` means descending; only `sortable` fields |
| `search` | `golang` | over `searchable` fields |
| `filter[status][in]` | `draft,published` | only allowed filters and operators (below) |
| `include` | `category,tags` | eager loading of relations; the target resources' permissions apply too |
| `summarize` | `views.sum,views.avg` | summaries over every filtered record; only items in `capabilities.summaries` |

For included relations, `ViewAny`, tenant, `Scope` and `View` of the target are checked. For a model without a normal resource registration, `Config.IncludeTargets` must introduce a read-only resource and the list of allowed JSON fields; that setting does not create CRUD routes or a relation manager. A model or field with a custom `MarshalJSON` is not accepted in this allowlist, so the projection cannot be bypassed.

Response:

```json
{
  "data": [
    { "id": 1, "title": "...", "status": "draft", "published_at": "2026-01-02T21:00:00Z", "category": { "id": 3, "name": "..." } }
  ],
  "meta": { "page": 1, "per_page": 25, "total": 134, "last_page": 6 },
  "abilities": {
    "1": { "update": true, "delete": false, "actions": { "publish": true } }
  }
}
```

`abilities` covers each record on this page, keyed by the record as a string. With `summarize` the response also has `"summary": { "views": { "sum": 145667, "avg": 2387.98 } }` (`null` when there are no records).

### ✅ `POST /resources/{resource}/reorder`

For resources with a manual ordering column (`capabilities.reorderable`, e.g. `"sort_order"`):

```json
{ "records": [3, 1, 2] }
```

- The records take the ordering values they already have, redistributed in the new order; records that are not sent stay where they are. So sending only the moved part is enough.
- If the values are all the same (all 0, say), it starts from the smallest and goes up one by one.
- A duplicate record ← 400; a missing record or one outside the scope ← 404; without a `Reorder` rule (with a gate: the `{resource}.reorder` permission) or without `Update` on each ← 403. `abilities.reorder` in meta says whether the ordering button is shown.
- Only the ordering column is updated: no `updated_at` and no hooks.
- Sorting by `sort=sort_order` is allowed even when it is not in `sortable`.

If a parameter outside the allowlist is sent, the response is **400** and it is not silently ignored, so mistakes show up early.

**Filters:** `filter[column]=v` is the same as `filter[column][eq]=v`.

| Operator | Meaning | Value |
|---|---|---|
| `eq` | `=` | one value |
| `in` | `IN (...)` | several values separated by `,` (at most 100) |
| `gt`, `gte`, `lt`, `lte` | `>`, `>=`, `<`, `<=` | one value |
| `null` | `true`: `IS NULL`, `false`: `IS NOT NULL` | nullable columns only |

- The value is converted to the column type: boolean (`true`/`false`/`1`/`0`), number, and date (ISO 8601, e.g. `2026-11-18T20:30:00Z`). A value that does not convert ← 400.
- The UI builds a date range from the user's local days: "from" is midnight of that day (`gte`) and "to" is midnight of the next day (`lt`), both as UTC instants.
- **Custom filters** are names, not columns: `filter[mine]=1` or `filter[tags][in]=1,4`. Only `eq` and `in` (the same); values reach the server code as they are, and the server can reject them with 400. Their names are in meta: `capabilities.filters`.

### ✅ `GET /resources/{resource}/records/{id}`

Response: `{ "data": { ... }, "abilities": { "update": true, "delete": false, "actions": { ... } } }` — it also accepts the `include` parameter. Create and update responses carry `abilities` too.

### ✅ `POST /resources/{resource}/records`

Body: `{ "data": { "title": "...", "category_id": 1 } }` ← **201** with `{ "data": { ... } }`

- Fields outside `fillable` are not ignored: they return a **422** (protection against mass assignment).
- Only the submitted fields (and fields the server hooks set, such as the author) are inserted, so the database default applies to the rest (like Eloquent).
- Server hooks can reject the save with a 422 and a field error (like validation rules).
- After the save the response is read from the database again and shows the value actually stored.
- Writing many-to-many relations (such as `tags`) is added in phase 6.

### ✅ `PATCH /resources/{resource}/records/{id}`

Partial update. Response: `{ "data": { ... } }`

- Only the submitted fields are validated and saved. A `required` rule for a field that was not sent does not run.
- The `unique:table,column` rule automatically ignores the current record.
- `updated_at` is updated.

### ✅ `DELETE /resources/{resource}/records/{id}`

Only for `Deletable` resources. Response: **204**

- `BeforeDelete` can reject with a 422 (`{ "message": "...", "errors": {} }`).
- A record that a foreign key points at: **409** ("this record is used elsewhere…").

---

## ✅ Actions

### `POST /resources/{resource}/actions/{action}`

```json
{ "records": [1, 2, 7], "data": { "category_id": 3 } }
```

- `records`: the selected records; for a header action (`without_records`) it must be empty, otherwise at least one. Cap: 500.
- `data`: the modal form's values; validated with the action's own `Rules` (422 with `errors`).
- If one of the records is not found: **404** (an action never runs on fewer records than were selected).
- The action runs inside a transaction; an error means rollback. `Reject` ← 422 with a message.

Response:

```json
{
  "notification": { "type": "warning", "title": "2 posts were published; 1 post had no body." },
  "redirect": null,
  "open_url": null,
  "invalidate": ["posts"]
}
```

- `notification.type`: `success`, `warning`, `danger`, `info`
- `invalidate`: resources the UI must reload (default: this resource)
- `redirect`: a path inside the panel (optional)
- `open_url`: a public URL on the same origin for leaving the panel, such as a short-lived preview; the UI refuses a URL with a different origin or a non-HTTP(S) protocol.

**Bulk delete** is a built-in action: `POST /resources/{resource}/actions/delete` with `records`; all in one transaction, and if `BeforeDelete` rejects one of them, none is deleted.

A resource's actions appear in `/meta`: `"deletable": true, "actions": [{ "name": "publish", "without_records": false, "fields": [] }]`.

---

## ✅ Relation options (for selects)

### `GET /resources/{resource}/relations/{relation}/options`

| Parameter | Notes |
|---|---|
| `search` | search over the target resource's `Searchable` (or its `TitleColumn`) |
| `page` | 25-per-page pages, ordered by `TitleColumn` |
| `values` | labels for the given keys, e.g. `values=1,4` for a select's current value; no pagination |

```json
{ "data": [ { "value": 3, "label": "Programming" } ], "meta": { "page": 1, "has_more": false } }
```

- The relation must be in `Relations` and the target model must itself be a registered resource with a `TitleColumn`; otherwise 404 (e.g. `author`, which is not a resource).
- The user must be allowed to view both resources (403), and the target's `Scope` applies.

### `GET /resources/{resource}/options`

The same parameters and response, but the options are the records of `{resource}` itself (key and `TitleColumn`). It is for values that point at another resource without being an ORM relation, such as `page_id` inside a menu's JSON items. In TSX: `f.select('page_id').optionsFrom('pages')`.

- The user must have `ViewAny` on that resource (403); `Scope` and the current tenant apply, so only records that the same resource's list would show come back.
- A resource without a `TitleColumn` ← 400.
- This endpoint does not guarantee that the stored value is valid: the holding resource's server must check the value itself (like the menu hook that rejects another team's page).

### ✅ Relation managers: the related records of one record

Only for has-many and many-to-many relations (the kind and keys are read from the ORM schema itself).

| Path | Work |
|---|---|
| `GET /resources/{resource}/records/{id}/relations/{relation}/records` | list; the same parameters as the related resource's list (`sort`, `search`, `filter`, `include`) |
| `POST .../relations/{relation}/records` | create a related record with `{ "data": {...} }`; the related resource's validation and hooks run |
| `POST .../relations/{relation}/attach` | `{ "records": [3, 7] }` many-to-many only; items already attached are left alone |
| `POST .../relations/{relation}/detach` | `{ "records": [3] }` many-to-many only; the records themselves stay |

The list response is like a normal list, plus:

```json
"relation": { "type": "has_many", "foreign_key": "category_id", "create": true, "attach": false, "detach": false }
```

- In has-many, the server fills `foreign_key` with the parent key, whatever the client sent.
- Editing and deleting a related record go through the related resource's own routes.
- Permissions: `ViewAny` on both resources and `View` on the parent are required (403); a parent outside the scope ← 404. The related resource's scope applies to the list and to attach/detach records as well (a record outside it ← 404). Create, attach and detach need `Update` on the parent, and create needs `Create` on the related resource.

### Writing many-to-many relations

If a relation of list type (such as `tags`) is in `Fillable`, it can be sent as keys:

```json
{ "data": { "title": "...", "tags": [1, 4] } }
```

- The relation is replaced with exactly those keys (`[]` or `null` clears it), in the same save transaction.
- Validation uses the usual rules: `"tags": "array"` and `"tags.*": "exists:tags,id"`. An error for one member comes back under the `tags.0` key.
- To read it back: `include=tags`.
- Many-to-many only; the pivot rows are written directly and the related records themselves are untouched.

---

## ✅ File uploads

### `POST /resources/{resource}/uploads/{field}`

`multipart/form-data` with a `file` field ← **201**. For an existing record send `?record={id}`: the upload then needs permission to update that record; without it, permission to create.

```json
{ "token": "upl_…", "name": "cover.png", "size": 5631, "mime": "image/png" }
```

1. The file is uploaded as soon as it is chosen and kept in temporary space (the `local` disk, private).
2. The form sends the `token` in that same field instead of the file (`"cover": "upl_…"`).
3. On save, the server moves the file under a random name onto the destination disk and folder and stores its **path** (`covers/<random>.png`).

Rules (server side, `Uploads` in Config):

- The file type is detected from its **content**, not its name or header; the extension is derived from that type too. Default: JPEG, PNG, WebP, GIF. SVG does not count as an image (it can carry a script).
- A size above `MaxSize` (default 2 MB) ← 422.
- The token is valid only for that user, resource and field, for 24 hours; it is consumed on use.
- A string value other than a token is accepted only when it equals the field's current value, so a client can never point the field at an arbitrary file. `null` clears the field.
- The previous file (replaced or cleared) and the files of a deleted record are removed; if the save fails, the moved file is removed.
- Temporary files older than 24 hours are removed automatically.
- **Multiple files** (`Multiple: true`): the field value is an array of paths. On save each member is either a fresh token or one of the field's current paths (duplicates are not allowed); more than `MaxFiles` ← 422. Current files that are not in the new list are removed after the save.

## ✅ Dashboard

### `GET /widgets/{name}`

One dashboard widget's data: `{ "data": ... }`. The look (stats, chart) is set by the TSX definition with the same name; the server supplies only the numbers.

```json
// stats
{ "data": { "posts": { "value": 61, "change": 0.12, "trend": [3, 5, 8] } } }
// chart
{ "data": { "labels": ["2026-10", "2026-11"], "datasets": [{ "key": "published", "data": [9, 14] }] } }
```

- `change` is relative to the previous period (`0.12` means 12% more); `trend` runs from old to new.
- Date labels are ISO (`2026-11-19` or `2026-11`, UTC); the UI formats them.
- Without permission (`Authorize`) ← 403; unknown widget ← 404. Meta lists the allowed widgets: `"widgets": ["overview", ...]`.
- The table widget has no API of its own: it is the resource list.

## ✅ Notifications

Only when `meta.panel.notifications` is non-empty: `{ "poll": 15 }` (seconds).

| Path | Work |
|---|---|
| `GET /notifications?page=1` | the user's notifications, newest first (20 per page) |
| `POST /notifications/{id}/read` | read one |
| `POST /notifications/read` | read all |
| `DELETE /notifications/{id}` | delete one (another user's notification ← 404) |
| `DELETE /notifications` | delete all |

```json
{
  "data": [{ "id": 3, "type": "success", "title": "...", "body": "...", "url": "/posts/9", "read_at": null, "created_at": "2026-11-19T14:45:20Z" }],
  "meta": { "page": 1, "has_more": false, "unread": 1 }
}
```

- `type`: `success`, `warning`, `danger`, `info`. `url` is always a path inside the panel (the server accepts nothing else).
- The UI asks again every `poll` seconds (only while the tab is visible) and when returning to the tab.

## ✅ Custom pages

| Path | Work |
|---|---|
| `GET /pages/{name}` | `{ "data": ... }` from the page's `Data` |
| `POST /pages/{name}` | `{ "data": {...} }` ← validated with `Rules` and run through `Handle` in a transaction; the response is like an action (`notification`, `invalidate`, `redirect`, `open_url`) |

Without permission ← 403; unknown page ← 404; `Reject` or `ValidationErrors` ← 422. Meta lists the allowed pages: `"pages": ["authors"]`.

## ✅ Global search

`GET /search?q=go` ← `{ "data": [{ "resource": "posts", "key": 8, "title": "Introducing Go", "details": { "status": "published" } }] }`

- Only `GlobalSearch` resources, in `Searchable` columns, at most 5 results each.
- ViewAny, scope and view of each resource apply. Fewer than 2 characters ← an empty list; more than 100 characters ← 400.
- `meta.search` says whether the user has anything to search (so the UI can show the search box).

## ✅ Multi-tenancy

- Every request sends the `X-Mks-Tenant: {id}` header (the UI keeps it per tab). Without it, the user's first tenant.
- A tenant the user is not a member of ← **403** for every request.
- `meta.tenancy`: `{ "tenants": [{ "id": 1, "name": "Engineering" }], "current": 1 }`, or `null` without multi-tenancy.
- Resources with `Tenant`: only the current tenant's records (outside it ← 404); the server fills the tenant column; a reference to another tenant's record ← 422 on that field.

## ✅ Plugins

- A plugin's server-side routes live under `/plugins/{name}/...`, with the same session, CSRF and login as the rest of the API.
- A plugin's resources, pages and widgets use the same routes and rules as the rest; no new protocol is needed.

## ✅ Permissions

The rules are set by the server (in the Goravel adapter: `mks.Policy[T]`). `abilities` only hides buttons; the server checks again on **every** request.

| Rule | Where it is checked | When absent |
|---|---|---|
| `ViewAny` | all resource routes; meta | 403, and the resource does not appear in meta |
| `View` | `GET /records/{id}` | 403 |
| `Create` | `POST /records`, uploads without `record` | 403 |
| `Update` | `PATCH`, uploads with `record`, record actions without `Authorize` | 403 |
| `Delete` | `DELETE` and bulk delete | 403 |
| `Scope` | list, record, update, delete, actions, relation options | a record outside it does not exist: 404 |

- A bulk action runs only when it is allowed on **all** records; otherwise 403 and nothing changes.
- `abilities` in meta: `create`, `delete` (whether the delete button makes sense at all), `reorder` (manual reordering) and `header_actions`.
- `abilities` per record: `update`, `delete` and `actions` (record actions).
- Without `update` the UI shows the form read-only ("view"); without `create` there is no "new" button.

## ✅ JSON fields

JSON columns (in Go, `mks.JSON[T]`) are **the value itself** in the API, not a string:

```json
{ "keywords": ["go", "react"], "meta": { "seo_title": "..." },
  "faqs": [{ "question": "...", "answer": "..." }],
  "blocks": [{ "type": "quote", "data": { "text": "...", "author": "..." } }] }
```

- An empty value in the database (`NULL`) comes back as `null`.
- Nested rules with `*`: `"faqs.*.question": "required"`. Errors carry the exact path: `faqs.1.answer`, `blocks.0.type`. These rules run only when the list has members.
- Builder format: an array of `{ type, data }`.

## ✅ Rich text (HTML)

`HTML` fields in Config are sanitized on save with bluemonday (the UGC policy): formatting, links, lists, headings and images survive; scripts, styles, events (`onerror`…) and `javascript:` are removed.

---

## Phase 0 decisions (2026-11-19)

The backend and frontend prototypes settled these decisions:

| Topic | Decision |
|---|---|
| ids | the backend's own type, not always a string |
| dates | ISO 8601 in UTC; the adapter serializes records itself because Carbon's default JSON has no time zone |
| an included but empty relation | `[]` or `null`, never dropping the key |
| create | only the submitted fields are inserted; the response is read from the database |
| update | partial PATCH; `unique` ignores the current record |
| uniqueness race error | 409 instead of 500 (tested with concurrent requests) |
| filter format | `filter[col][op]=v` in the query string is enough. Combined AND/OR filters were deferred to phase 7 |
| cursor pagination | deferred to phase 7; numeric pagination for now |
| server-side form hooks | designed in phase 4 |
| protocol versioning | in phase 1 alongside the `meta` endpoint (a `protocol` field in the response) |
| table summaries and grouping | designed in phase 7 |

### Implementation notes (Goravel adapter)

- Gin's `QueryMap` drops the operator in `filter[status][in]`, so the adapter reads the raw query string.
- In Goravel validation rules, `regex:` swallows the rest of the string and must be the last rule.
- Whitelist column names are checked with a regex when a resource is registered, because they enter SQL as identifiers. Values are always bound as parameters.
- Uniqueness error detection is implemented for MySQL, PostgreSQL and SQLite.
