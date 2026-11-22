# @mksine/mks

[![npm version](https://img.shields.io/npm/v/@mksine/mks)](https://www.npmjs.com/package/@mksine/mks)
[![npm downloads](https://img.shields.io/npm/dm/@mksine/mks)](https://www.npmjs.com/package/@mksine/mks)
[![CI](https://github.com/MiranSalehi/mks/actions/workflows/ci.yml/badge.svg)](https://github.com/MiranSalehi/mks/actions/workflows/ci.yml)
[![Go reference](https://pkg.go.dev/badge/github.com/MiranSalehi/mks.svg)](https://pkg.go.dev/github.com/MiranSalehi/mks)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/MiranSalehi/mks/blob/main/LICENSE)

The admin panel for [Goravel](https://goravel.dev): a Filament-style dashboard with
resources, forms, tables, actions, policies and multi-tenancy, plus the MKSine CMS
(pages, posts, media, menus, page builder, themes, ZIP plugins and SEO).

One repository ships both halves, in lockstep:

| Half | Distributed as | For |
|---|---|---|
| UI (React 19 + TypeScript) | npm package `@mksine/mks` | your frontend build (Vite, …) |
| Backend (Go) | Go module `github.com/MiranSalehi/mks` | your Goravel application |

The UI talks to the backend over a small, documented JSON protocol
([PROTOCOL.md](https://github.com/MiranSalehi/mks/blob/main/docs/PROTOCOL.md)); the
server stays the source of truth for data, validation and permissions.

## Features

- **Resources** — list, create, edit and view screens from one definition; searchable,
  sortable and filterable columns with whitelists enforced server-side.
- **Forms** — sections, grids, tabs, repeaters, builders, relation managers, media
  pickers, rich text (Tiptap), code (CodeMirror), date pickers (Jalali and Gregorian),
  conditional visibility and validation.
- **Tables** — filters, badges, summaries, bulk actions, reordering, exports.
- **Actions** — row and bulk actions with confirmation, forms, modals and hooks.
- **Access control** — Goravel policies plus a panel-wide gate, roles and permissions,
  per-resource abilities, and multi-tenancy scoped by the panel's tenant.
- **Infolists, widgets and pages** — read-only record views, dashboard widgets, custom
  pages and clusters.
- **CMS** — pages, posts, categories, tags, comments, media library, menus, reusable
  sections, page builder, themes, plugin ZIPs, shortcodes and an SEO analyzer.
- **i18n** — English and Persian out of the box, RTL/LTR, custom locales and per-install
  translation overrides.
- **Typed** — `mks:types` generates TypeScript types from the Go resources; every subpath
  ships `.d.ts` declarations.

## Requirements

- **Node 22+** and pnpm (or npm)
- **React 19** and the TanStack packages listed under peer dependencies
- For the Go half: **Go 1.26+** (required by the package's dependency graph — see
  [the README](https://github.com/MiranSalehi/mks#requirements)) and **Goravel 1.18+**

## Install

### UI

```bash
pnpm add @mksine/mks
pnpm add react react-dom @tanstack/react-query @tanstack/react-router @tanstack/react-table
```

### Go backend

```bash
./artisan package:install github.com/MiranSalehi/mks
```

This registers `mks.ServiceProvider`, publishes `config/mks.go` and installs the core
notifications migration. Then bind the application pieces the panel cannot know (the admin
guard, the user model, the built UI and the resources) — the complete walkthrough, with the
exact files and a troubleshooting table, is in
[Installing MKS](https://github.com/MiranSalehi/mks/blob/main/docs/INSTALL.md).

## Quick start (UI)

```tsx
import { createRoot } from 'react-dom/client'
import { Panel, defineResource, type ResourceDefinition } from '@mksine/mks'
import { cms } from '@mksine/mks/cms'
import '@mksine/mks/styles.css'

interface Post {
  id: number
  title: string
  status: 'draft' | 'published'
}

const posts: ResourceDefinition<Post> = defineResource<Post>({
  name: 'posts',
  label: 'Post',
  pluralLabel: 'Posts',
  form: (f) => [
    f.textInput('title').required().maxLength(255),
    f.select('status').options({ draft: 'Draft', published: 'Published' }),
  ],
  table: (t) => [t.text('title').searchable().sortable(), t.badge('status')],
  actions: (a) => [a.edit(), a.delete().requiresConfirmation()],
})

createRoot(document.getElementById('root')!).render(
  <Panel basePath="/admin" brand="MKS" resources={[posts]} plugins={[cms()]} />,
)
```

The backend for the same resource is a Go definition (`mks.Define[models.Post](…)`) — the
two halves are documented together in
[the architecture guide](https://github.com/MiranSalehi/mks/blob/main/docs/ARCHITECTURE.md).

## Subpaths

| Import | What it is |
|---|---|
| `@mksine/mks` | the panel core: resources, forms, tables, actions, filters, widgets, protocol |
| `@mksine/mks/access` | roles and permissions |
| `@mksine/mks/activity` | activity log |
| `@mksine/mks/appearance` | themes and appearance |
| `@mksine/mks/cms` | the MKSine CMS: pages, posts, media, menus, builder, themes, SEO |
| `@mksine/mks/navigation` | admin menu editor |
| `@mksine/mks/system` | system health and background processes |
| `@mksine/mks/translations` | panel translations |
| `@mksine/mks/styles.css` | the panel stylesheet |

## Tailwind

Import the package stylesheet after Tailwind:

```css
@import "tailwindcss";
@import "@mksine/mks/styles.css";
```

## Documentation

- [Installing MKS — the complete setup for a Goravel app](https://github.com/MiranSalehi/mks/blob/main/docs/INSTALL.md)
- [README — install, wiring and commands](https://github.com/MiranSalehi/mks#readme)
- [Architecture — how the two halves split and the resource API](https://github.com/MiranSalehi/mks/blob/main/docs/ARCHITECTURE.md)
- [Protocol — the JSON contract](https://github.com/MiranSalehi/mks/blob/main/docs/PROTOCOL.md)
- [Publishing](https://github.com/MiranSalehi/mks/blob/main/docs/PUBLISHING.md)
- [Changelog](https://github.com/MiranSalehi/mks/blob/main/CHANGELOG.md)

## Versioning

The npm package and the Go module are released together under the same `0.x` version
(Git tag `vX.Y.Z`). The JSON protocol is v0: minor releases may include breaking changes,
and they are listed in the changelog. Pin exact versions in production until `1.0`.

## Contributing and security

- [Contributing](https://github.com/MiranSalehi/mks/blob/main/CONTRIBUTING.md)
- [Security policy](https://github.com/MiranSalehi/mks/blob/main/SECURITY.md) — report
  vulnerabilities privately, not in a public issue.

## License

MIT © [Miran Salehi](https://github.com/MiranSalehi)
