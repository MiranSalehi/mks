# Publishing a release

Maintainer notes for cutting an MKS release. The npm packages and the Go module share a
version but are published separately.

## One-time setup

Only the repository owner can do these:

1. **Make the GitHub repository public.** `go get` and npm provenance both need a public
   repository (a private repository requires `GOPRIVATE` and authenticated users).
2. **Own the npm scope** (`@mks`). Create the organization on npm and invite the
   maintainers, or rename the packages to a scope you own **before** the first release.
3. **Configure publishing.** Either:
   - add an `NPM_TOKEN` repository secret (automation token with publish rights), or
   - use npm trusted publishing (OIDC) and remove the token from
     `.github/workflows/release.yml`.
4. Create a protected `npm` environment in GitHub (Settings → Environments) for the
   release workflow.
5. Enable branch protection on `main` and require the CI workflow.

## Every release

1. Update the version, e.g. `0.1.0` → `0.1.1` (this is the repository root and the
   package):

   ```bash
   grep -n '"version": "0.1.0"' package.json
   # edit it by hand or use changesets
   ```

   The Go module has no version in `go.mod`; its version is the tag (`vX.Y.Z`), so
   the npm version and the tag must stay in lockstep.

2. Add the release notes at the top of `CHANGELOG.md` and move the previous
   `Unreleased` entries under the new version.

3. Run the full checks:

   ```bash
   pnpm install
   pnpm typecheck
   pnpm test
   pnpm build
   pnpm check:exports
   pnpm pack:check
   go vet ./... && go test -race ./...
   ```

4. Inspect a tarball before publishing:

   ```bash
   tar -tzf /tmp/mks-packs/mks-mks-0.1.1.tgz | sort
   ```

   It must contain `package/dist/**`, `package/styles.css`, `package/LICENSE`,
   `package/README.md` and `package/package.json` — and nothing from `src/`,
   `node_modules/` or the tests.

5. Commit, tag and push:

   ```bash
   git add -A
   git commit -m "release: 0.1.1"
   git tag v0.1.1
   git push origin main --tags
   ```

   The module is the repository root, so the plain `vX.Y.Z` tag serves both: the Go
   proxy picks it up and the same tag triggers the npm workflow.

6. The `Release` workflow publishes `@mks/mks` with `--provenance`. To publish from
   a machine instead:

   ```bash
   pnpm build
   pnpm publish --access public --provenance
   ```

7. Verify in a clean directory:

   ```bash
   mkdir /tmp/mks-check && cd /tmp/mks-check
   go mod init check
   go get github.com/MiranSalehi/mks@v0.1.1
   npm init -y
   pnpm add @mks/mks@0.1.1
   ```

## If something goes wrong

- npm: `npm deprecate @mks/mks@0.1.1 "reason"` (a published version cannot be reused).
- Go: fix the tag by deleting and re-pushing it **before** it is cached by the proxy;
  after that, publish a new patch version.
- Never re-tag a version that users may already have imported.
