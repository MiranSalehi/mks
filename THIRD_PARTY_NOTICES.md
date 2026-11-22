# Third-party notices

MKS is MIT-licensed. It uses the following third-party projects at runtime; each
remains under its own license (the package manager ships their license files).

## Bundled fonts

- **Vazirmatn** (`@fontsource-variable/vazirmatn`) — SIL Open Font License 1.1.
  The OFL requires that the font not be sold on its own and that its name not be
  changed; the license text is in the distributed package.

## Direct dependencies (main)

| Package | License |
|---|---|
| `react`, `react-dom` | MIT |
| `@tanstack/react-query`, `@tanstack/react-router`, `@tanstack/react-table` | MIT |
| `@tiptap/*` (rich editor) | MIT |
| `@codemirror/*`, `@lezer/highlight` (code editor) | MIT |
| `marked` (markdown) | MIT |
| `dompurify` | Apache-2.0 OR MPL-2.0 |
| `tailwindcss` (peer/dev) | MIT |

## Go dependencies

The Go module depends on `github.com/goravel/framework`, `gorm.io/gorm`,
`github.com/microcosm-cc/bluemonday` and `github.com/stretchr/testify` (plus their
transitive dependencies), all under MIT, BSD or Apache-2.0 licenses. Run
`go list -m -json all` for the full module list.

If you redistribute a built admin bundle, keep the license texts of the bundled
dependencies that your bundler includes.
