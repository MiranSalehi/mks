# MKS

[![CI](https://github.com/MiranSalehi/mks/actions/workflows/ci.yml/badge.svg)](https://github.com/MiranSalehi/mks/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/@mks/mks)](https://www.npmjs.com/package/@mks/mks)
[![Go reference](https://pkg.go.dev/badge/github.com/MiranSalehi/mks.svg)](https://pkg.go.dev/github.com/MiranSalehi/mks)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

An admin panel for Goravel: a React/TypeScript UI, a Go backend, and a language-neutral JSON protocol between them.

The repository is the package itself: the Go module `github.com/MiranSalehi/mks`
**and** the npm package `@mks/mks` ship from the same tree — the Filament-style core
plus the MKSine CMS and first-party plugins.

The plugins live in the same npm package as subpath exports (`@mks/mks/cms`,
`@mks/mks/access`, …); nothing is installed separately.

There is no demo application here. Wire the two halves into your own Goravel app: the Go module receives the models and permissions, and `@mks/mks` renders the panel from the resource definitions you write in TSX.

## Requirements

- Go 1.26 or newer — the floor comes from the package's dependency graph, not from
  Goravel (v1.18 itself needs Go 1.25): the CMS image pipeline imports
  `golang.org/x/image` v0.48, which declares `go 1.26.0`
- Node 22 or newer and pnpm
- MySQL 8 or newer (or PostgreSQL/SQLite) — for your app's data, not for the package itself

## Install

The Go half is a Goravel package with a service provider and an installer:

```bash
./artisan package:install github.com/MiranSalehi/mks
```

This registers `mks.ServiceProvider` in `bootstrap/providers.go`, publishes
`config/mks.go`, and installs the core notifications migration (registered in
`bootstrap/migrations.go`). The frontend is one npm package:

```bash
pnpm add @mks/mks
```

The panel's UI is your own frontend build: a Vite (or any bundler) entry that imports
`@mks/mks` and renders `<Panel>`. Ship that build to the Go server one of two ways:

- **Embed it.** `go:embed` the build output and bind that `fs.FS` at `mks.UIBinding`.
- **Serve it from disk.** `./artisan mks:assets` copies the build (default `admin/dist`)
  into `public/vendor/mks`; bind `os.DirFS("public/vendor/mks")` at `mks.UIBinding`. A
  frontend change then lands without a Go rebuild, and the command warns when the Go
  module and the installed `@mks/mks` versions differ.

> Until a tagged release is available, use a local path in `go.mod`:
>
> ```go
> require github.com/MiranSalehi/mks v0.1.0
> replace github.com/MiranSalehi/mks => ../mks
> ```

The panel cannot know an application's user model, resources or build, so the
application binds a few small pieces in its own service provider:

```go
// app/providers/mks.go
type MKSProvider struct{}

func (*MKSProvider) Register(app foundation.Application) {
    // A fresh user model for the panel's auth guard.
    app.Singleton(mks.UserBinding, func(foundation.Application) (any, error) {
        return mks.UserFactory(func() mks.User { return &models.User{} }), nil
    })
    // The built frontend (go:embed).
    app.Singleton(mks.UIBinding, func(foundation.Application) (any, error) {
        return adminui.FS(), nil
    })
    // Optional: change options before the panel is created (tenancy, …).
    app.Singleton(mks.OptionsBinding, func(foundation.Application) (any, error) {
        return mks.OptionsHook(func(o *mks.Options) {
            o.Tenancy = &mks.TenancyOptions{Tenants: userTeams}
        }), nil
    })
    // Resources and plugins.
    app.Singleton(mks.ConfigureBinding, func(foundation.Application) (any, error) {
        return mks.ConfigureHook(func(p *mks.Panel) {
            p.Register(posts)
            p.Use(&cms.Plugin{Orm: app.MakeOrm()})
        }), nil
    })
}

func (*MKSProvider) Boot(foundation.Application) {}
```

Register `&mks.ServiceProvider{}` (installed above) and `&MKSProvider{}` in
`bootstrap/providers.go`. After boot, `facades.MKS()` (from
`github.com/MiranSalehi/mks/facades`) returns the panel, for example
in application commands. The types an app or a plugin implements (`Plugin`,
`Resource`, `User`, hooks, options) are also exported from
`github.com/MiranSalehi/mks/contracts`, the conventional Goravel
`contracts/` import path.

Until `MKSProvider` is registered the application keeps working — artisan commands
included — and the panel path answers `503` with the missing piece. This matters
right after `package:install`, before the user model is bound.

The `mks:types` command is registered by the package; application-specific commands
such as `mks:resource` stay in `bootstrap/commands.go`.

If your application installs its own router fallback (for an SPA or a custom 404),
set `mks.fallback` to `false` / `Options.NoFallback: true` and call
`panel.ServeUI(ctx)` from that fallback. A router has one fallback: whichever is
registered last wins, and the panel's client-side routes would otherwise 404 after
a reload.

The installer creates the core notifications table. The first-party plugins expose
their own tables as `CreateTable(schema.Schema)` functions (for example
`cms.CreateTable`); add those calls to your application's migrations when you enable
the plugins.

## Manual wiring (advanced)

The service provider above is the standard path. Constructing the panel directly is
still supported for tests and unusual setups — for example when the panel must exist
before Goravel's providers boot:

**Go** — a panel, its resources and plugins:

```go
p := mks.New(mks.Options{
    Path:       "/admin",
    Orm:        facades.Orm(),
    Validation: facades.Validation(),
    Auth:       mks.AuthOptions{ /* guard, hash, new user, rate limiter */ },
    Storage:    facades.Storage(),
    UI:         uiFS, // the built client, embedded with go:embed
})

p.Register(myResource)                 // your panel.Define[models.Post](cfg)
p.Use(&cms.Plugin{Orm: facades.Orm()}) // the first-party plugins you want
p.Routes(facades.Route())
```

**TSX** — the client for those resources (see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)):

```tsx
import { Panel, defineResource } from '@mks/mks'
import { cms } from '@mks/mks/cms'
import '@mks/mks/styles.css'

createRoot(document.getElementById('root')!).render(
  <Panel basePath="/admin" brand="MKS" resources={[posts]} plugins={[cms()]} />,
)
```

**Tailwind** — import the package styles after Tailwind:

```css
@import "tailwindcss";
@import "@mks/mks/styles.css";
```

## Commands

The Go module adds these artisan commands to your app (`mks` category):

| Command | What it does |
|---|---|
| `mks:types` | Writes TypeScript types for the registered resources |
| `mks:assets` | Copies the built frontend from `@mks/mks` into the app (`--from`, `--to`, `--prune`) |
| `mks:resource <Model> --model-import=...` | Generates starter Go config and a TSX resource from a model; `--model-import` is the import path of your own models package |
| `mks:cms-media-copies` | Makes the smaller image copies the CMS uses (needs the cms plugin) |

## Development

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build            # tsup + tsc declarations for every package
pnpm check:exports    # exports/entries/plugins agree (after build)
pnpm pack:check       # what npm would publish
go test -race ./... && go vet ./...
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — how the two halves split, the resource API, permissions, plugins
- [Protocol](docs/PROTOCOL.md) — the JSON contract every adapter implements
- [Publishing](docs/PUBLISHING.md) — how a release is cut
- [Changelog](CHANGELOG.md) — what changed, per release

## Versioning

The npm packages and the Go module are released together under `0.x` versions. The JSON
protocol is still v0: minor releases may include breaking protocol or API changes, and
they are listed in the changelog. Pin exact versions in production until `1.0`.

## Security

Please report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © Miran Salehi. Third-party notices are in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
