# Changelog

Changes worth noting for each release are recorded here.

## Unreleased

### Fixed

- **The application boots without the panel.** `mks.ServiceProvider.Boot` no longer panics when the panel cannot be built (usually because `mks.UserBinding` is not bound yet): it logs the reason, keeps artisan commands working after `package:install`, and answers the panel path with `503` instead of taking every command down.
- **The router fallback is optional.** `mks.fallback` / `Options.NoFallback` turn off the panel's catch-all, so an application with its own fallback (an SPA, a custom 404) can call `panel.ServeUI` from it; a router has one fallback and the last one wins, which silently broke the panel's client-side routes.
- **Session is a declared dependency** in the provider's `Relationship()`, matching the session middleware the panel mounts.
- **A protocol mismatch is visible.** The UI shows a banner when the server's protocol differs from its own instead of only writing to the console, and `mks:assets` warns when the Go module and `@mksine/mks` versions differ.

### Added

- `mks:assets` copies the application's built panel (default `admin/dist`) into `public/vendor/mks` (`--from`, `--to`, `--prune`), for apps that serve it with `os.DirFS` instead of `go:embed`.
- `contracts/` is the conventional import path for `Plugin`, `Resource`, `User`, hooks and options.
- `pnpm check:exports` (CI and release) fails when `exports`, `src/plugins` and the tsup entries drift; the tsup entries now live in `tsup.config.ts`.

### Changed

- The npm package moved to the `@mksine` scope: it is now `@mksine/mks` (the Go module
  path is unchanged). It was never published under the old name.
- The source is English-only: comments, server messages, default content, page
  templates, theme text and UI defaults are translated. The Persian locale itself stays
  (`src/locales/fa.ts`, the `persian` table in `i18n.go`, `tr({ fa, … })`, `L(fa, …)`,
  `T(fa, …)`), because that is the translation data, not source text. Persian/Arabic
  punctuation in input handling (digit conversion, `،` separators in CSV/regex parsing)
  is kept for the same reason.
- Jalali month and weekday names use Latin transliterations now, and the site's dates
  and counters use Latin digits (the panel's fa messages still show Persian digits).
- The repository is now the package: `git init` was reset to this directory, the old
  multi-package history was dropped, and the Go module path became
  `github.com/MiranSalehi/mks` (was `.../packages/mks`). The npm and Go releases share
  the plain `vX.Y.Z` tag.
- GitHub Actions CI and release workflows are back — at the repository root this time —
  running the Go and UI checks on every push and tag.
- The Go module is tidy: `golang.org/x/image` is a direct dependency.
- The npm package exposes `types` at the top level and `./package.json`.

## 0.1.0 — 2026-11-22

First public release. One package is published as the Go module
`github.com/MiranSalehi/mks` and the npm package `@mksine/mks`
(with the first-party plugins as subpath exports), all under the MIT license.

The repository is the package itself: the Go module `github.com/MiranSalehi/mks` and
the npm package `@mksine/mks` ship from the same tree, at its root.

### Added

- Goravel package structure: `mks.ServiceProvider` (relationships, container binding, routes, commands, publishes), `setup/setup.go` for `artisan package:install`, a publishable `config/mks.go`, a core notifications migration registered in the application, and `facades.MKS()`.
- The project is named **MKS**: the npm packages are `@mksine/*`, the Go module is `github.com/MiranSalehi/mks`, the commands are `mks:*` (`mks:types`, `mks:resource`, `mks:cms-media-copies`, `mks:geo-import`), the panel's own CSS classes and browser keys are `mks-*`, and the brand is MKS.
- Rich editor like MKSine's CKEditor 5 (built on Tiptap): the same toolbar in the same order (Insert Media, Insert shortcode, undo/redo, Source, Show blocks, Paragraph and Styles dropdowns, font size, family, color and background, bold/italic/underline, link with Downloadable, table with a size grid and a column/row/merge toolbar, highlight markers and pens, block quote, code block with languages, alignment, bulleted/numbered lists with styles, start and reversed order, to-do list, indent/outdent), with items that do not fit under "⋮". Core: `addRichEditorButton` for plugin buttons, `richEditor().placeholder()` and `.height()`. The server keeps the editor's colors, sizes, fonts, alignment, named styles, to-do lists, video and audio and still drops everything else. CMS: `DescribeShortcode` and the `cms_shortcodes` meta list the shortcodes for the editor (ZIP plugins can give `label`, `description`, `example`).
- Media picker like MKSine's: a large "Media library" window with search, a type filter, a drop zone that uploads several files, a 3–7 column grid, an "Attachment details" column (name, type, dimensions, size, upload date), "Showing … to … of …" pages and a footer with the selected count, Cancel and Confirm selection. One or several files are chosen and confirmed with the button. The field shows preview tiles and MKSine's dashed "Select / Change / Add media" button; galleries number their files and reorder by drag or arrows. The page builder no longer crashes when a block with a canvas error is opened.
- Page builder like MKSine's: toolbar (component count, templates, undo/redo, full screen, add component), device widths, and a canvas that is the page as the server draws it with each block framed (name, edit, up, down, copy, delete) and "+" rows between blocks and inside columns; a side panel edits a block (Content and Block style tabs); the add dialog has MKSine's categories, icons, names and descriptions; blocks drag anywhere, between columns too.
- Menu builder like MKSine's (`menu_builder`, opened from the menus list): sources (custom link, categories, tags, pages, posts) on one side, the menu as a flat list on the other with drag to reorder, sideways drag or ←/→ to nest, edit and remove, and "Select menu" / "Save menu".
- Core: custom fields get nested `errors` and `get`; `PanelForm` takes `actions` and `initialErrors` and no longer submits an outer form; link actions to panel paths (with `#hash`) use the router.
- ZIP plugins can declare content types (`content_types` in plugin.json): activating the plugin adds their list and form screens to the panel and their archive and single pages to the site; deactivating removes them, and deleting the plugin with its data removes their entries. Core: `Panel.SetResources(group, ...)` adds and removes resources while the app runs (copy-on-write, safe under concurrency), and a plugin's `metaResources` builds their screens from meta.
- Site menus nest up to six levels (MKSine's menus have any depth); groups may sit at any level. The menu form moves an entry under the one before it or back out (core `Repeater.tree()`), and the site's header and footer show nested submenus.
- Sidebar like MKSine's (WordPress style): each group is one row whose items open in a panel beside the sidebar on hover or focus (a click opens the first item); a menu search box; a collapsed icon rail with the same panels; groups that open in place in the mobile drawer.
- CMS: `addSeoCheck` adds checks to the SEO analysis (MKSine's `mksine.seo.analysis_checks`); library images inside post, page and entry bodies get srcset or WebP `<picture>` sources, size and lazy loading.
- The admin is laid out exactly like MKSine's: the same sidebar groups, items, order and labels; a "Settings" cluster (General, Appearance, Permalinks, Performance, Admin menu, Extensions); "View site" and "My appearance" in the user menu; a dashboard with the account widget. The playground's demo content that MKSine does not have (demo blog, authors report, activity log, "project" type, stats widgets, sample hooks) was removed (migration `drop_demo`); import/export and the hooks page are opt-in (`ImportExport`, `HooksPage`).
- Core navigation: `navigationSort`, `navigation: false`, ordered groups with icons (`navigationGroups`), extra sidebar links (`navigationLinks`), clusters (`clusters`, `definePage({ cluster })`), page `visible`, `userMenuItems`, plugin `onMeta`, the `w.account()` widget, header link actions (`a.link().header()`), and FormData bodies in `api.plugin`.
- CMS menu locations like MKSine (`cms_menu_locations`): a location shows one menu and a menu can be at several; locations come from code, themes and plugins.
- Themes and plugins from ZIP files (no marketplace): theme and plugin manager pages with upload, update (newer or forced), rollback, activate/deactivate, logs and delete. Themes bring Go templates, CSS, JavaScript and files; plugins bring builder blocks, template hook HTML, shortcodes, site settings, menu locations, CSS and JavaScript. No server code runs from a package. ZIPs are checked for size bombs, path tricks, links, hidden files and unknown file types; templates are parsed before install; a plugin that fails to start is turned off.
- The geo plugin (countries, states, cities, the "Locations and addresses" settings tab and `/api/geo`) was built and then removed as not needed; migration `20261121000019_drop_geo` drops its tables and settings.
- Site menus for CMS sites (now in the `cms` plugin): a `menus` resource per site with a header or footer location and a two-level item tree (page, link, group) in one JSON column, rendered on the public `/sites/{team}/{slug}` pages. Only pages that are public at request time are linked; unsafe link schemes are refused on save and on render.
- `Select.optionsFrom(resource)` and `GET /resources/{resource}/options`: select options from the records of another resource, within its permissions, scope and tenant.
- MKSine port as first-party plugins (Go `packages/mks/plugins/*`, UI `@mksine/mks/*`):
  - `cms`: multi-site CMS where a site is a panel tenant. Pages (plain or block-based, revisions, trash, preview, copy), posts, nested categories, tags, comments with moderation and a public form, media library with resized variants and a picker, site menus, reusable sections and page templates, a 20-block page builder with live preview, custom content types, per-site settings and permalinks, Go themes with custom CSS/JS, hooks, filters and shortcodes, live SEO analysis with share tags, JSON-LD, `sitemap.xml` and `robots.txt`, and a versioned public page cache.
  - `access`: roles and permissions on the panel gate; users cannot grant permissions they do not hold.
  - `navigation`: panel menu editor (hide, rename, regroup, reorder).
  - `translations`: panel text editor stored in the database.
  - `system`: health checks and a console limited to allowlisted artisan commands, run without a shell.
- Core: a global permission gate (`Panel.SetGate`, `Permission`, `Permissions`, `Allowed`) that applies to every resource ability, action, widget and page on top of policies.
- Appearance plugin: the panel look (preset, primary color, mode, sidebar, corners, text size, density, form field style: outlined, filled, underline or floating labels, and labels above or beside fields) for the company, each user, and up to 24 named looks; core `setAppearance`/`useAppearance`.
- Hooks page (CMS): turn event listeners, filters and template actions off or reorder them, for every site.
- Console: background runs with live output and stopping; custom pages can show a sidebar count (`Page.NavigationBadge`), used for System health problems.
- User profiles (playground) and access roles on the user form (`access.Plugin.SetUserRoles`); `mks:cms-media-copies`.
- Builder (core): drag a block by its card header anywhere in the tree, into another column or card or back out; drop zones appear only where it may go (`moveBlock`).
- CMS media: WebP copies like MKSine (`photo-640w.webp` plus one at the image's own width, quality 80) when the plugin has a WebP encoder (`Plugin.WebP`; `cms.CWebP("")` runs libwebp's cwebp without a shell and is nil when it is not installed). Pages serve them in `<picture>` with the original as fallback; WebP uploads get copies and can be edited; `mks:cms-media-copies --webp` converts older copies. Copies of EXIF-rotated JPEGs are now upright, and their stored width and height match how they show.
- CMS builder: grid column widths per screen size (MKSine `column_spans`): a 12-column grid with a span for phones and for 640, 768 and 1024 px and up, and when columns stack (always, tablet, phone, never). Grids without widths keep the automatic layout.
- CMS builder: click a block in the live preview to open it in the form (nested blocks too); core `focusBuilderBlock`.
- CMS builder: MKSine's seven default-theme blocks (domain hero, product showcase, service cards, featured domains, icon feature grid, testimonials grid, a live feed of approved comments) and the "MKSine theme home" template.
- CMS media: an image editor action (crop with free, 16:9, 4:3 or 1:1 ratios, rotate, mirror; EXIF orientation honored) that replaces the file or saves a new one, remaking the resized copies. Core: custom fields get the record (`CustomFieldProps.record`), and a single-record action's form sees its record.
- Translations plugin: managers add panel languages (code, name, direction, a base language for untranslated texts) and translate them; the UI offers them in the user menu (core: `setCustomLocales`, `baseLocale`).
- CMS: content import and export (posts, pages, media details) from CSV, JSON, XML and WordPress exports, with column mapping, dry runs and per-row errors; core `ActionResult.Data` carries results to page UIs.
- Core: reordering has its own permission (`{resource}.reorder`, `Policy.Reorder`, `abilities.reorder`); before, update was enough.
- Core: `Select.createOption(resource, field)` (create a missing option from the search text), `Builder.history()` (undo/redo, JSON copy/paste, duplicating blocks), `ModelResource.AfterRead` (fill computed fields of listed, shown and saved records), `Panel.Path()`, and `?tenant=` in panel links.
- CMS: tags created from the post and page pickers, post counts on categories and tags, post and page galleries (`cms_media_attachments`, `mediaGallery` field), and a front-end admin bar for site members (never cached).
- Core: `Config.NavigationBadge` (a count next to the resource in the sidebar, within the user's scope), list tabs over a filter (`tabs`), and `HookContext.AfterCommit`.
- Core: `Panel.MetaValue` (`meta.extra`), `HookContext.Tenants()` / `BelongsTo()`, `Upload.NameField`, safe video and audio upload types, React content in `Placeholder`, exported `Modal`, and `ApiClient.plugin()`.

- Admin menu editor rebuilt to MKSine's layout: one row per sidebar entry, grouped by its default group, with columns for the default group, placement (default, hidden or another group), target group, order and custom name. Every row has an editor and a hide/show toggle, plus "Edit groups" and "Back to default" in the header, and each action saves at once and redraws the sidebar. Core: item overrides carry `placement`, `groupNavigation()` applies it, and `usePanel().refreshMeta()` refetches meta after a plugin changes the sidebar.

### Security and reliability

- Choosing a menu for a menu location failed with a server error, and location changes did not clear the site's page cache.
- Grouped policy predicates so `OR` conditions cannot escape tenant, record-key, or relation constraints; unsupported query operations in `Policy.Scope` fail before SQL execution.
- Restrict read-only relation includes to an explicit JSON field allowlist, rejecting custom `MarshalJSON` implementations that could bypass projection.
- Preserve existing many-to-many links the caller cannot see, reject inaccessible submitted targets, and deduplicate submitted relation keys.
- Resolve submitted relation keys through the configured resource key before writing join rows, including GORM relations whose referenced model column differs from the public key.
- SQLite relation-sync regressions verify allowed custom-key selection, rejection without join mutation for an inaccessible submitted key, and clearing only visible custom-key links while retaining hidden links; existing scope-composition and transaction-rollback cases remain covered.
- CMS shortcodes never run inside HTML tags, so an attribute value cannot close its quotes; unsafe builder links become `#`; JSON-LD escapes `</script>`.
- CMS custom JavaScript needs `Plugin.CustomJS` (off by default): on the panel's origin it could call the panel API as any user who opened the site.
- CMS page cache: keyed by origin when `BaseURL` is unset (no Host-header poisoning), only valid page numbers are cached, caches are retired after the commit, and cached pages never outlive the next scheduled publication.
- CMS author pages exist only for authors with a public post on the site; `Plugin.AuthorAllowed` limits post authors. Upload names stored by `NameField` are capped at 255 characters.
- Playground users: current team only; changing or deleting a user needs all of that user's permissions (`access.Plugin.Outranks`).
- Publish times are truncated to whole seconds, because MySQL rounds fractional seconds up and a just-published page could 404 for a second.

### Performance

- Load resource lists, dashboard, and create/edit/view screens as route chunks instead of placing all of them in the initial UI entry. In the playground build, the entry dropped from 608.77 kB to 514.20 kB (15.5%). Compared with the list-only split, gzip size dropped from 164.01 kB to 156.68 kB. The list chunk is 61.69 kB; dashboard is 7.42 kB; create/edit/view are separate chunks. The entry still exceeds Vite's 500 kB warning threshold. A smaller entry does not demonstrate a reduction in total JavaScript or browser load times; cold-load and route-transition measurements remain outstanding.

### Validation

- The checks are Go tests with the race detector, UI tests, TypeScript checks, the production build of the client and a Playwright Chromium smoke test with a mocked API; all run locally.
- Full backend Playwright/E2E, full multi-tenant database validation, performance profiling, release/versioning policy and the project-level license decision remain open before a release.

### Removed

- The "collapse the menu" button at the foot of the sidebar: the topbar button already does it, in both directions.
- The users' unused second "work role" column (migration `20261121000023_drop_user_role`): since the access plugin, every permission comes from a user's roles, so the column only repeated them on the user form and had to be kept in step by hand. The form now has one roles field, and the users list shows those roles by name (`role_names`).
- The GitHub Actions workflow (`.github/workflows/ci.yml`) was removed on the project owner's decision: CI is not needed for now, so every check runs locally. Its last hosted run had failed on the playground Go tests, which expect a `.env` and database the workflow did not provide.
