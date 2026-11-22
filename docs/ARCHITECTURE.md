# Architecture

## Main principle: separation of responsibilities

| | Frontend (React) | Backend (Go) |
|---|---|---|
| **Responsible for** | the user interface and its logic | data and security |
| **Defines** | fields, columns, layout, actions, `visible`/`disabled`, dependent fields | the model, fillable fields, validation rules, fields allowed for search/sort/filter, relations, permissions |
| **Trust** | nothing from the client is trusted | the source of truth |

The result: reactivity is instant (no round trip to the server, unlike Livewire) and security stays entirely on the server.

**Security rule:** anything the frontend declares (for example "this column is sortable") is only a request. The server executes only what it has put in its own whitelist.

## Example resource definition

### Frontend (`resources/posts.tsx`)

```tsx
import { defineResource } from '@mks/mks'
import type { Post } from '../generated/types' // generated from the backend

export default defineResource<Post>({
  name: 'posts',
  label: 'Post',
  pluralLabel: 'Posts',
  icon: 'document-text',

  form: (f) => [
    f.section('Main information').schema([
      f.textInput('title').required().maxLength(255),
      f.select('type').options({ post: 'Post', page: 'Page' }),
      f.textInput('slug').visible(({ get }) => get('type') === 'post'),
      f.select('category_id').relationship('category', 'name').searchable(),
    ]),
  ],

  table: (t) => [
    t.text('title').searchable().sortable(),
    t.badge('status').colors({ draft: 'gray', published: 'success' }),
    t.date('created_at').sortable(),
  ],

  actions: (a) => [a.edit(), a.delete().requiresConfirmation()],
})
```

### Backend (`app/admin/post_resource.go`)

```go
type PostResource struct {
    mks.BaseResource[models.Post]
}

func (PostResource) Name() string { return "posts" }

func (PostResource) Fillable() []string   { return []string{"title", "type", "slug", "category_id", "status"} }
func (PostResource) Searchable() []string { return []string{"title"} }
func (PostResource) Sortable() []string   { return []string{"title", "created_at"} }

func (PostResource) Rules(ctx mks.Context) map[string]string {
    return map[string]string{
        "title":       "required|max_len:255",
        "category_id": "required|exists:categories,id",
    }
}

func (PostResource) Relations() []mks.Relation {
    return []mks.Relation{mks.BelongsTo("category", "categories", "name")}
}

func (PostResource) Policy() mks.Policy { return policies.PostPolicy{} }
```

(The final API for permissions is `mks.Policy[T]`; see the "Permissions" section.)

> The APIs above are drafts, finalized in phase 0 along with the first prototype.

## Reactivity and hooks

**In the browser** (no round trip to the server, unlike Livewire):

```tsx
f.textInput('title').afterStateUpdated(({ value, old, get, set, operation }) => {
  if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) set('slug', slugify(String(value)))
}),
f.toggle('is_featured').disabled(({ get }) => get('status') !== 'published'),
f.textarea('body').required(({ get }) => get('status') === 'published'),
f.placeholder('word_count').content(({ get }) => countWords(get('body'))),
```

**On the server**, for anything that needs the user, the database or the saved record:

```go
mks.Define[models.Post](cfg).
    BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
        data["author_id"] = ctx.UserID() // not fillable; only the server sets it
        return nil
    }).
    BeforeUpdate(func(ctx *mks.HookContext, post *models.Post, data map[string]any) error {
        if data["status"] == "published" && emptyBody(post, data) {
            return mks.ValidationErrors{"body": "A post body is required to publish."}
        }
        return nil
    }).
    AfterSave(func(ctx *mks.HookContext, post *models.Post, created bool) error {
        return ctx.Query.Create(&models.Activity{...}) // same transaction; an error means rollback
    })
```

- Before hooks run after the fillable and validation checks and can change `data`.
- In an update, `data` holds only the changed fields; for rules that depend on several fields use `record` (Goravel rules only see submitted fields).
- Create/update and `AfterSave` share one transaction.

## Actions

```tsx
// TSX: where it appears and what it asks
actions: (a) => [
  a.make('publish').label('Publish').row().bulk().requiresConfirmation(),
  a.make('change_category').bulk().form((f) => [f.select('category_id').required().relationship('categories', 'name')]),
  a.delete().visible((post) => post.status !== 'published'),
]
```

```go
// Go: what it does (typed by model, inside the transaction)
.Actions(mks.Action[models.Post]{
    Name:  "change_category",
    Rules: map[string]string{"category_id": "required|exists:categories,id"},
    Handle: func(ctx *mks.ActionContext[models.Post]) (*mks.ActionResult, error) {
        // ctx.Records ([]*models.Post), ctx.Data, ctx.Query (transaction), ctx.User
        return mks.Success("..."), nil
    },
})
```

## Filters

```tsx
// TSX: which filters exist and how they are shown
filters: (f) => [
  f.select('status').options(statusLabels).multiple(),          // filter[status][in]=draft,archived
  f.select('category_id').relationship('category').multiple(),  // options from the server, searchable
  f.ternary('published_at').nullable().labels('has', 'does not have'),  // filter[published_at][null]=false
  f.dateRange('created_at'),                                    // gte and lt, with Jalali days
  f.toggle('mine').label('Only my posts'),                      // custom server filter
]
```

```go
// Go: what is allowed, and conditions that are not a single column
Filterable: map[string][]string{"status": {"eq", "in"}, "published_at": {"gte", "lt", "null"}},
// ...
.Filters(mks.Filter{Name: "mine", Apply: func(ctx *mks.HookContext, q orm.Query, _ []string) (orm.Query, error) {
    return q.Where("author_id = ?", ctx.UserID()), nil
}})
```

## Multi-tenancy

```go
mks.Options{Tenancy: &mks.TenancyOptions{Tenants: userTeams}} // the user's teams; the first is the default
mks.Config{Name: "posts", Tenant: "team_id", ...}                // the rest is automatic
// in a widget or page, a manual query: ctx.Query.Model(&Post{}).Where("team_id = ?", ctx.TenantID())
```

```tsx
<Panel ... tenantLabel="Team" />  // the picker in the sidebar, kept separately per tab
```

## Plugins

```tsx
// a custom field and column: your own component, in any form and table
export const colorPicker = defineField({ component: ColorControl, defaults: { swatches: [...] }, empty: null })
export const colorSwatch = defineColumn({ component: ColorSwatch, defaults: { showHex: true } })
form: (f) => [f.textInput('name'), colorPicker<Category>('color').label('Color')]

// one package: resources, pages, widgets and a topbar button
export const activityLog = () => definePlugin({ name: 'activity-log', resources: [activities], widgets: [...] })
// <Panel ... plugins={plugins} />
```

```go
// the server half, with the same name
type Plugin struct{}
func (Plugin) Name() string { return "activity-log" }
func (Plugin) Register(p *mks.Panel) {
    p.OnSaved(func(ctx *mks.HookContext, resource string, record any, created bool) error { ... }) // in the transaction
    p.Register(mks.Define[Activity](...))
}
// p.Use(activity.Plugin{})
// the plugin's own routes: implement Routes(r route.Router) ← {panel}/api/plugins/{name}/...
```

### Menu badge, list tabs and AfterCommit

```go
mks.Config{Name: "comments", NavigationBadge: func(ctx *mks.HookContext, q orm.Query) orm.Query {
    return q.Where("status = ?", "pending") // over the records the user sees (tenant and scope)
}, NavigationBadgeColor: "warning"}

ctx.AfterCommit(func() { cache.Forget(key) }) // in hooks: only if the transaction committed

// computed fields (gorm:"-") for a list, a single record and the save response, with one query per page
mks.Define[Category](cfg).AfterRead(func(ctx *mks.HookContext, cats []*Category) error { ... })
```

```tsx
defineResource({ ..., tabs: { filter: 'status', tabs: [{ label: 'All' }, { label: 'Pending', value: 'pending' }] } })
f.select('tags').relationship('tags').multiple().createOption('cms_tags') // "Create …" for new search text
f.builder('blocks').history() // undo, redo, JSON copy/paste
```

After every write the UI fetches meta again so the counts are fresh.

### Plugin-wide data in meta

```go
p.MetaValue("navigation", func(ctx *mks.HookContext) any { return config }) // ← meta.extra.navigation
```

The UI reads values from `meta.extra`; the sidebar applies `extra.navigation` (the menu editor) and i18n applies `extra.translations` (the text editor) itself. In TSX, plugins get `api.plugin(name)` for their own routes.

### First-party plugins

The plugins ship in the same package: a Go package at `plugins/<name>` and a frontend subpath `@mks/mks/<name>`.

| Plugin | What it does | Permissions |
|---|---|---|
| `cms` | multi-site CMS (site = tenant): pages, posts, categories and tags, comments, media, menus, sections, page builder, custom content types, settings and permalinks, themes, hooks and shortcodes, SEO, page cache, public site | `cms.*` |
| `access` | roles and permissions on the gate, with no way to grant more than you hold | `roles` resource permissions |
| `navigation` | panel menu editor | `navigation.manage` |
| `translations` | panel text editor (in the database) | `translations.manage` |
| `system` | system health and a console for allowlisted artisan commands (foreground or background) | `system.health`, `system.console` |
| `appearance` | panel appearance: company look, per-user look and named looks | `appearance.manage` |
| `activity` | activity log (sample plugin) | — |

The package has one `styles.css` (with `@source "./dist";`) that the app imports into its CSS so Tailwind generates the classes of the core and every plugin.

## Custom pages

```tsx
// TSX: a React component with data and a form from the server
function AuthorsPage() {
  const data = usePageData<{ authors: Author[] }>()
  const submit = usePageSubmit()
  return (
    <Card heading="Message to everyone">
      <PanelForm schema={(f) => [f.textInput('title').required()]} onSubmit={submit} />
    </Card>
  )
}
export default definePage({ name: 'authors', label: 'Authors', group: 'Reports', component: AuthorsPage })
// <Panel ... pages={[authors]} />
```

```go
// Go: who may see it, what data it gets, what the form does
p.Pages(mks.Page{
    Name: "authors", Authorize: notViewer,
    Data:   func(ctx *mks.WidgetContext) (any, error) { ... },
    Rules:  map[string]string{"title": "required|max_len:120"},
    Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) { ... },
})
```

## Notifications

```go
// in Options: Notifications: true (and the migration from mks.CreateNotificationsTable)
// inside an action or AfterSave, in the same transaction:
err := ctx.Notify(mks.Notice{
    Type: "success", Title: "Your post was published", URL: fmt.Sprintf("/posts/%d", post.ID),
}, post.AuthorID)
// outside a request (a job, say): p.Notify(notice, userIDs...)
```

The UI builds the bell and the drawer itself; no TSX definition is needed.

## Dashboard

```tsx
// TSX: the look
export default defineDashboard({
  widgets: (w) => [
    w.stats('overview').stats((s) => [s.make('posts').label('Posts').icon(<DocumentIcon />)]),
    w.chart('published_per_month').type('line').formatLabels('month').dataset('published', 'Published'),
    w.table<Post>('latest_posts').resource('posts').sort('-created_at').limit(5),
  ],
})
// <Panel ... dashboard={dashboard} />
```

```go
// Go: the numbers and the permission
p.Widgets(mks.Widget{Name: "published_per_month", Data: func(ctx *mks.WidgetContext) (any, error) {
    labels, values, err := mks.CountPer(ctx.Query.Model(&models.Post{}).Where("status = ?", "published"),
        &models.Post{}, "published_at", mks.Month, 12, ctx.Now)
    return mks.Chart{Labels: labels, Datasets: []mks.Dataset{{Key: "published", Data: values}}}, err
}})
```

## Infolist (view page)

```tsx
infolist: (i) => [
  i.section('Content').schema([i.text('title').weight('semibold'), i.html('body')]),
  i.section('Questions').schema([
    i.repeatable('faqs').schema((q) => [q.text('question'), q.text('answer')]), // q: the item type (FAQ)
  ]),
  i.section('Status').aside().columns(1).schema([
    i.badge('status').colors({ published: 'success' }),
    i.date('published_at').visible((p) => p.status === 'published'),
    i.list('tags').title('name'),
  ]),
],
```

This is display only: the data comes from the same `GET /records/{id}` and the relations the entries need are included from their paths. Nothing is needed on the server.

## Table features

```tsx
table: (t) => [
  t.text('views').numeric().toggleable().summarize('sum', 'avg'),  // a hideable column, with summaries
  t.date('created_at').toggleable(true),                           // hidden at start
],
groups: (g) => [g.make('status').titles(statusLabels), g.make('category_id').title('category.name')],
```

```go
Summaries:   map[string][]string{"views": {"sum", "avg"}}, // allowed summaries
Reorderable: "sort_order",                                 // manual ordering (POST /reorder)
```

## Relation managers

```tsx
// TSX (categories.tsx): below the edit form, the table of this category's posts
relations: (r) => [
  r.manager<Post>('posts').label('Posts').table((t) => [t.text('title').sortable(), t.badge('status')]),
]
```

```go
// Go: the relation on the model and in Config; the kind (has-many or many-to-many) and the keys come from the gorm tags
Posts []*Post `gorm:"foreignKey:CategoryID"`   // models.Category
Relations: map[string]string{"posts": "Posts"} // Config
```

The create form is the related resource's own form; editing goes to the record's own page.

## Permissions

```go
mks.Define[models.Post](cfg).Policy(mks.Policy[models.Post]{
    Create: func(ctx *mks.HookContext) bool { return role(ctx) != "viewer" },
    Update: func(ctx *mks.HookContext, p *models.Post) bool { return role(ctx) == "admin" || p.AuthorID == ctx.UserID() },
    Delete: func(ctx *mks.HookContext, _ *models.Post) bool { return role(ctx) == "admin" },
    // records outside Scope do not exist for this user (404)
    Scope: func(ctx *mks.HookContext, q orm.Query) orm.Query {
        if role(ctx) == "viewer" { return q.Where("status = ?", "published") }
        return q
    },
})
```

- Every rule is optional; a rule that is not set allows.
- `Scope` may only add `WHERE` conditions (including `Scopes` built from `WHERE` conditions); using any other query operation before the SQL runs panics.
- `OR` conditions are wrapped in a group so they cannot bypass the tenant, record-key or relation constraint.
- Each action can have its own `Authorize`; without it, a record action relies on `Update`.
- The server passes the outcome to the UI as `abilities` so disallowed buttons are hidden and the form becomes read-only. TSX holds no permission rules; Go is the only source of truth.

### Global gate (roles)

A resource policy is the per-record rule; the gate is a second, global layer (for example roles from the `access` plugin). When a gate is set, every ability needs the gate permission on top of the policy:

```go
p.SetGate(func(ctx *mks.HookContext, permission string) bool { return roles.Has(ctx.UserID(), permission) })
p.Permission("cms.settings.manage")     // a custom permission, in the role form's permission list
p.Allowed(ctx, "cms.settings.manage")   // a manual check in a page, widget or action
p.Permissions()                         // all permissions, grouped (for the role editor)
```

- Names: `posts.view_any|view|create|update|delete`, `posts.action.publish`, `widget.{name}`, `page.{name}` and custom permissions.
- `Register` wraps each resource in a decorator that combines policy and actions with the gate; the gate never allows what the policy denied.
- Without a gate, everything relies on the policy alone, as before.

## Avoiding duplicated work

Defining the fields in both Go and TS risks drift. The solution:

1. The backend publishes the resource structure through `GET /meta` (fields, types, relations, capabilities).
2. `./artisan mks:types` generates **TypeScript types** from those same Go models (`admin/types.generated.ts`), with no HTTP and no login needed.
3. As a result `f.textInput('titel')` (a typo) fails at compile time.
4. In development mode the UI warns when a column is `sortable()` but the server has not allowed it.

## Technology choices

### Frontend

| Need | Choice | Reason |
|---|---|---|
| Language and build | TypeScript + Vite | standard, fast |
| Rendering | React 19 | |
| Routing | TanStack Router | type-safe, suited to an SPA |
| Data fetching | TanStack Query | cache, invalidation, retry |
| Table | TanStack Table (headless) | table logic without imposing a look |
| Form state | a dedicated engine over a light store (Zustand) | Filament-like reactivity (`get`/`set`, dependencies) with the fewest re-renders |
| Base components | Radix UI / Base UI (headless) | accessibility (a11y) and RTL support |
| Styling | Tailwind CSS v4 | theme system with CSS variables |
| Calendar | Jalali support in the DatePicker | the Persian market |

### Backend

| Need | Choice |
|---|---|
| Language | Go 1.26+ |
| Framework | Goravel v1.18 (HTTP driver: Gin) |
| ORM | Goravel's own ORM (on GORM) |
| Playground database | local MySQL (no Docker); the adapter must also be tested on PostgreSQL and SQLite |
| Panel authentication | session + secure cookie + CSRF (not JWT in localStorage) |
| Deployment | the built UI embedded with `embed` in the Go binary — a single executable |

## Repository layout

```
mks/                          # repository root = the package
├── src/                      # frontend: the core and src/plugins/*
├── plugins/                  # Go half of the plugins (cms, access, navigation, …)
├── facades/                  # facades.MKS()
├── contracts/                # the types an app or a plugin implements
├── setup/                    # artisan package:install installer
├── docs/                     # architecture, protocol, publishing
├── go.mod                    # github.com/MiranSalehi/mks
├── package.json              # @mks/mks
└── styles.css                # built from src/styles.css (not committed)
```

- Frontend: one npm package, `@mks/mks`, with the plugins as subpath exports
- Go: the module at the repository root; it needs nothing else
- The application that uses the package lives in its own repository; it supplies the models, the migrations, the built UI (`go:embed`) and the wiring

## Request flow (example: listing posts)

```
Browser                                   Go server
  │  GET /admin/posts  (TanStack Router)   │
  │  ── GET /admin/api/resources/posts/records?page=1&sort=-created_at&search=go ──▶
  │                                         │ 1. authenticate (session)
  │                                         │ 2. policy: viewAny
  │                                         │ 3. sort/search/filter in the whitelist?
  │                                         │ 4. ORM query + pagination
  │  ◀── { data: [...], meta: { total, ... } } ──
  │  render the table from table() in TSX   │
```

## Recorded decisions

| Date | Decision | Reason |
|---|---|---|
| 2026-11-19 | rewrite from scratch instead of forking Filament | design freedom; client-side reactivity |
| 2026-11-19 | UI in TSX, data and security in the backend | no round trip for UI logic |
| 2026-11-19 | protocol independent of the backend language | Laravel and Rust adapters possible later |
| 2026-11-19 | backend: Go + Goravel | development speed, good performance, memory safety, a familiar Laravel-like structure |
| 2026-11-19 | SPA + JSON API instead of Inertia | Go backend, UI embedded in the binary |
| 2026-11-19 | local MySQL for development, no Docker | Docker Hub is not reachable from the development network; MySQL is already installed |
| 2026-11-21 | MKSine features as first-party core plugins, not app code | reuse in any app; the core stays small |
| 2026-11-21 | CMS site = panel tenant | the same tenant security boundary for all site data |
| 2026-11-21 | themes as Go code instead of a loadable folder | no execution of uploaded code |
| 2026-11-21 | themes and plugins are installed from ZIP, no marketplace; packages carry no server code | the project owner wants ZIP uploads; Go cannot load code at runtime, so a package is declarative (Go templates, blocks, hooks, shortcodes, settings, assets) |
| 2026-11-21 | the UI mirrors MKSine exactly | menus and screens like MKSine. The code samples in this document (posts, authors) are only samples |
| 2026-11-21 | the geo plugin (country, state, city) was removed | the system does not need it for now; its tables and settings are dropped by a migration |

## Installing as a Goravel package

The repository is a normal Goravel package: it has a `ServiceProvider` (bindings,
routes, commands, publishes), a `setup/setup.go` installer used by
`artisan package:install`, a publishable `config/mks.go`, a core notifications
migration, and a `facades.MKS()` accessor. The application binds the user model, the
built UI, the options hook and the resources/plugins hook; see the README install
section.

The service provider is the standard path; `mks.New` remains available for tests and
for applications that build the panel themselves.
