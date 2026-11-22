# Installing MKS in a Goravel application

This is the complete walkthrough for wiring MKS into a **fresh Goravel 1.18 application**:
the Go package, the admin guard, the user model, the application provider and the React
frontend. If you only do `package:install` you get a panel that answers `503`, because the
package cannot know your user model, your resources or your frontend build.

The same steps are what the test application in this repository does; `./artisan mks:doctor`
(step 8) checks them for you.

## Requirements

- Go **1.26+** (the package's dependency graph requires it; see the README)
- Goravel **1.18+** with the Gin driver
- Node **22+** and pnpm (for the panel's frontend)
- A database (MySQL 8+, PostgreSQL or SQLite)

## 1. Install the Go half

```bash
./artisan package:install github.com/MiranSalehi/mks
```

The installer:

- registers `&mks.ServiceProvider{}` in `bootstrap/providers.go`,
- publishes `config/mks.go`,
- copies the core notifications migration to `database/migrations/` and registers it in
  `bootstrap/migrations.go`.

Check that `config/mks.go` exists and that `bootstrap/providers.go` imports the SDK as
`github.com/MiranSalehi/mks`.

## 2. Make sure the database driver is installed

Goravel is modular: the driver you configure in `config/database.go` must also be registered
as a service provider. The default template ships PostgreSQL, but `DB_CONNECTION` in `.env`
may say `mysql`. In that case install the MySQL driver:

```bash
go get github.com/goravel/mysql@v1.18.0
```

Add it to `bootstrap/providers.go`:

```go
import "github.com/goravel/mysql"

// inside Providers():
&mysql.ServiceProvider{},
```

and add the connection to `config/database.go`:

```go
"mysql": map[string]any{
    "host":     config.Env("DB_HOST"),
    "port":     config.Env("DB_PORT"),
    "database": config.Env("DB_DATABASE"),
    "username": config.Env("DB_USERNAME"),
    "password": config.Env("DB_PASSWORD"),
    "charset":  "utf8mb4",
    "loc":      "Local",
    "singular": false,
    "prefix":   "",
    "via": func() (driver.Driver, error) {
        return mysqlfacades.Mysql("mysql")
    },
},
```

> **Two MySQL servers?** If a local MySQL listens on both `127.0.0.1` and `::1`, `localhost`
> may resolve to the wrong one. Set `DB_HOST=127.0.0.1`. The symptom is
> `Error 1049 (42000): Unknown database '…'` even though the database exists.

Create the database itself if it does not exist:

```bash
mysql -uroot -e "CREATE DATABASE goravel CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

## 3. Add an admin guard (session)

The panel logs in with its own guard. It must use the **session** driver, because the panel
mounts the session middleware. In `config/auth.go` add a guard (the provider can be reused):

```go
"guards": map[string]any{
    "user": map[string]any{
        "driver":   "jwt",
        "provider": "user",
    },
    // The admin panel.
    "admin": map[string]any{
        "driver":   "session",
        "provider": "user",
    },
},
```

`config/mks.go` already points at it:

```go
"auth": map[string]any{
    "guard":      "admin",
    "identifier": "email",
},
```

> **The JWT trap.** Goravel builds the application's **default** guard on every request,
> even when the panel uses `admin`. If `auth.defaults.guard` is a JWT guard and
> `JWT_SECRET` is empty, every `/admin/api/*` request fails with an empty `500` and the log
> says `authentication secret is missing or required`. Fix it with
> `./artisan jwt:secret`, or point `auth.defaults.guard` at the session guard. MKS logs a
> warning at boot when it detects this.

## 4. Add the user model, table and an admin user

The panel logs the application's user model in. The model implements two methods:

```go
// app/models/user.go
package models

import (
    mks "github.com/MiranSalehi/mks"
    "github.com/goravel/framework/database/orm"
)

type User struct {
    orm.Model
    Name     string `json:"name"`
    Email    string `json:"email"`
    Password string `json:"-"`
}

func (u *User) PanelPassword() string { return u.Password }

func (u *User) PanelUser() mks.UserInfo {
    return mks.UserInfo{ID: u.ID, Name: u.Name, Email: u.Email}
}
```

Create the table with a migration:

```go
// database/migrations/20261122000001_create_users_table.go
package migrations

import (
    "github.com/goravel/framework/contracts/database/schema"

    "goravel/app/facades"
)

type M20261122000001CreateUsers struct{}

func (r *M20261122000001CreateUsers) Signature() string { return "20261122000001_create_users" }

func (r *M20261122000001CreateUsers) Up() error {
    return facades.Schema().Create("users", func(t schema.Blueprint) {
        t.ID()
        t.String("name")
        t.String("email")
        t.String("password")
        t.Timestamps()
        t.Unique("email")
    })
}

func (r *M20261122000001CreateUsers) Down() error {
    return facades.Schema().DropIfExists("users")
}
```

Register it in `bootstrap/migrations.go`:

```go
&migrations.M20261122000001CreateUsers{},
```

Create the first admin (a seeder is the simplest way):

```go
// database/seeders/database_seeder.go
package seeders

import (
    "github.com/goravel/framework/facades"

    "goravel/app/models"
)

type DatabaseSeeder struct{}

func (s *DatabaseSeeder) Signature() string { return "DatabaseSeeder" }

func (s *DatabaseSeeder) Run() error {
    count, err := facades.Orm().Query().Model(&models.User{}).
        Where("email", "admin@example.com").Count()
    if err == nil && count > 0 {
        return nil
    }
    password, err := facades.Hash().Make("password")
    if err != nil {
        return err
    }
    return facades.Orm().Query().Create(&models.User{
        Name: "Admin", Email: "admin@example.com", Password: password,
    })
}
```

Register it in `bootstrap/seeders.go` and `WithSeeders(Seeders)` in `bootstrap/app.go`:

```go
// bootstrap/seeders.go
func Seeders() []seeder.Seeder {
    return []seeder.Seeder{&seeders.DatabaseSeeder{}}
}
```

## 5. Bind the application's pieces

Create `app/providers/mks.go`. It binds the user model, the built frontend and the
resources:

```go
package providers

import (
    "os"

    mks "github.com/MiranSalehi/mks"
    "github.com/goravel/framework/contracts/foundation"

    "goravel/app/models"
)

type MKSProvider struct{}

func (*MKSProvider) Register(app foundation.Application) {
    // The panel creates a fresh user per request.
    app.Singleton(mks.UserBinding, func(foundation.Application) (any, error) {
        return mks.UserFactory(func() mks.User { return &models.User{} }), nil
    })

    // The built frontend (public/vendor/mks, filled by `artisan mks:assets`).
    app.Singleton(mks.UIBinding, func(foundation.Application) (any, error) {
        return os.DirFS("public/vendor/mks"), nil
    })

    // Resources and plugins.
    app.Singleton(mks.ConfigureBinding, func(foundation.Application) (any, error) {
        return mks.ConfigureHook(func(p *mks.Panel) {
            p.Register(usersResource())
        }), nil
    })
}

func (*MKSProvider) Boot(foundation.Application) {}

func usersResource() mks.Resource {
    return mks.Define[models.User](mks.Config{
        Name:        "users",
        Searchable:  []string{"name", "email"},
        Sortable:    []string{"name", "email", "created_at"},
        Fillable:    []string{"name", "email", "password"},
        TitleColumn: "name",
        DefaultSort: "name",
        Deletable:   true,
        Hashed:      []string{"password"}, // hashed on create and update
    })
}
```

Register it in `bootstrap/providers.go`, after the package provider:

```go
&mks.ServiceProvider{},
&providers.MKSProvider{},
```

`mks.Config.Hashed` hashes those fields with the application's `facades.Hash()` **after**
validation and after the resource's Before hooks. Leave the field out (or empty) on update
to keep the current password; do not hash it in a Before hook as well.

## 6. Add the frontend

The panel UI is your own Vite entry that imports `@mksine/mks`. Inside your Goravel
application:

### 6.1 Dependencies

```bash
pnpm add @mksine/mks react react-dom \
  @tanstack/react-query @tanstack/react-router @tanstack/react-table
pnpm add -D vite @vitejs/plugin-react @tailwindcss/vite tailwindcss typescript @types/react @types/react-dom
```

### 6.2 Vite config, entry and styles

`vite.config.ts` (repository root):

```ts
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  root: 'admin',
  base: '/admin/', // the panel is mounted here
  build: { outDir: 'dist', emptyOutDir: true },
  plugins: [react(), tailwindcss()],
  resolve: {
    dedupe: ['react', 'react-dom', '@tanstack/react-query', '@tanstack/react-router', '@tanstack/react-table'],
  },
  server: {
    port: 5173,
    proxy: { '/admin/api': 'http://127.0.0.1:3000' },
  },
})
```

`admin/index.html`:

```html
<!doctype html>
<html lang="en" dir="ltr">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>MKS</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="./main.tsx"></script>
  </body>
</html>
```

`admin/app.css`:

```css
@import "tailwindcss";
@import "@mksine/mks/styles.css";
```

`admin/main.tsx`:

```tsx
import { Panel } from '@mksine/mks'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './app.css'
import users from './resources/users'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Panel brand="MKS" basePath="/admin" resources={[users]} />
  </StrictMode>,
)
```

`admin/resources/users.tsx` mirrors the Go resource:

```tsx
import { defineResource } from '@mksine/mks'

export interface User {
  id: number
  name: string
  email: string
  created_at?: string
}

export default defineResource<User & { password?: string }>({
  name: 'users',
  label: 'User',
  pluralLabel: 'Users',
  table: (t) => [
    t.text('name').label('Name').searchable().sortable(),
    t.text('email').label('Email').searchable().sortable(),
    t.date('created_at').label('Created').since().sortable(),
  ],
  form: (f) => [
    f.textInput('name').label('Name').required().maxLength(255),
    f.textInput('email').label('Email').email().dir('ltr').required().maxLength(255),
    f.textInput('password').label('Password').password().dir('ltr')
      .required(({ operation }) => operation === 'create')
      .helperText('Leave empty on edit to keep it.'),
  ],
})
```

Add the scripts to `package.json`:

```json
"scripts": {
  "admin:dev": "vite",
  "admin:build": "vite build"
}
```

### 6.3 Build and serve

```bash
pnpm admin:build
./artisan mks:assets
```

`mks:assets` copies `admin/dist` (the default `--from`) into `public/vendor/mks` (the
default `--to`); `UIBinding` in `app/providers/mks.go` reads it with `os.DirFS`. Use
`--from`/`--to` if your build lives elsewhere, and `--prune` to delete hashed files from an
older build.

> **Prefer a single binary?** `go:embed` the same build output in a small package and bind
> that `fs.FS` at `mks.UIBinding` instead of `os.DirFS`. Everything else is unchanged.

## 7. Migrate, seed and run

```bash
./artisan migrate
./artisan db:seed
go run .
```

## 8. Verify

```bash
./artisan mks:doctor
```

It checks the user binding, the panel guard, the built UI, the core table and the npm
version, and prints what to fix. Then open `http://127.0.0.1:3000/admin` and log in with the
seeded user.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `/admin` answers `503` with “bind a user model factory” | `mks.UserBinding` is not bound | Add `MKSProvider` (step 5) |
| `/admin` answers `503` with “the UI is not built” | `UIBinding` is missing or the directory has no `index.html` | `pnpm admin:build && ./artisan mks:assets` |
| Every `/admin/api/*` request is a `500` with an empty body; log says `authentication secret is missing or required` | The default guard is JWT and `JWT_SECRET` is empty | `./artisan jwt:secret` (or make the default guard a session guard) |
| `404` on `/admin/api/auth/login` | Routes not mounted: the panel could not be built | `./artisan mks:doctor` |
| `404` on `/admin/assets/*` | Built with the wrong `base` | `base: '/admin/'` in `vite.config.ts`, then rebuild |
| `Unknown database '…'` while it exists | `localhost` resolves to another MySQL instance | `DB_HOST=127.0.0.1` |
| `not found database configuration` | `DB_CONNECTION` names a connection missing from `config/database.go` | Add the connection (step 2) |
| `Authentication guard [admin] is not defined` | Guard missing from `config/auth.go` | Step 3 |
| Login says the credentials do not match | The stored password is not a hash | Hash it (`facades.Hash().Make`) or use `Config.Hashed` |
| `pnpm add @mksine/mks` hangs or resolves a strange local path | pnpm points at a mirror registry that does not have the version | Check `pnpm config get registry`; put `registry=https://registry.npmjs.org/` in the app's `.npmrc` |
| pnpm refuses a brand-new version (`minimumReleaseAge`) | The registry policy waits before installing new releases | Approve it, or add the version to `minimumReleaseAgeExclude` in `pnpm-workspace.yaml` |

## Optional: the CMS

The CMS is a first-party plugin in the same package:

- Go: in `ConfigureHook`, `p.Use(&cms.Plugin{Orm: facades.Orm()})`, and add the CMS tables
  to a migration with `cms.CreateTable(facades.Schema())`.
- UI: `import { cms } from '@mksine/mks/cms'` and `plugins={[cms()]}` on `<Panel>`.

## Production notes

- Build the UI once and embed it (or keep `public/vendor/mks` in the deployment image);
  `mks:assets` runs at build time, not at runtime.
- Run `./artisan migrate` during deployment.
- Set `APP_DEBUG=false`; the panel sends error details only in debug mode.
- Keep the panel's session cookie secure (`SESSION_SECURE=true` behind HTTPS).
- Keep the Go module and `@mksine/mks` on the same version; the UI shows a banner when the
  panel protocol does not match the server.
