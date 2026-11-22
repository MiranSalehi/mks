// Fails when the published surface drifts: every "exports" subpath must have built files,
// every src/plugins/* plugin must be exported, and the tsup entries must cover both.
// Run after `pnpm build`; used by CI and the release workflow.
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const pkgDir = fileURLToPath(new URL('..', import.meta.url))
const pkg = JSON.parse(readFileSync(join(pkgDir, 'package.json'), 'utf8'))
const problems = []

const exports = pkg.exports ?? {}

// 1. Every target exists on disk (a stale/missing build, or a typo in the map).
for (const [subpath, target] of Object.entries(exports)) {
  if (subpath === './package.json') continue

  const files = typeof target === 'string' ? [target] : [target.types, target.import].filter(Boolean)
  for (const file of files) {
    const path = join(pkgDir, file)
    if (!existsSync(path)) problems.push(`${subpath} -> ${file} does not exist (run pnpm build)`)
  }

  if (typeof target === 'object' && !target.types) {
    problems.push(`${subpath} has no "types" target`)
  }
}

// 2. The plugins on disk and the exported subpaths are the same set.
const pluginDir = join(pkgDir, 'src', 'plugins')
const plugins = readdirSync(pluginDir, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
  .sort()

const exportedPlugins = Object.keys(exports)
  .filter((subpath) => subpath.startsWith('./') && subpath !== './styles.css' && subpath !== './package.json')
  .map((subpath) => subpath.slice(2))
  .sort()

for (const plugin of plugins) {
  if (!exportedPlugins.includes(plugin)) {
    problems.push(`src/plugins/${plugin} is not exported as "@mksine/mks/${plugin}"`)
  }
}
for (const plugin of exportedPlugins) {
  if (!plugins.includes(plugin)) {
    problems.push(`"@mksine/mks/${plugin}" is exported but src/plugins/${plugin} does not exist`)
  }
}

// 3. The tsup entries build exactly those subpaths.
const tsup = readFileSync(join(pkgDir, 'tsup.config.ts'), 'utf8')
const entries = [...tsup.matchAll(/'src\/([^']+)'/g)].map((match) => match[1])

if (!entries.includes('index.ts')) problems.push('tsup.config.ts does not build src/index.ts')
for (const plugin of plugins) {
  const entry = `plugins/${plugin}/index.tsx`
  if (!entries.includes(entry)) problems.push(`tsup.config.ts does not build src/${entry}`)
}

const styles = exports['./styles.css']
if (styles && !existsSync(join(pkgDir, styles))) {
  problems.push('./styles.css is exported but styles.css does not exist (run pnpm build)')
}

if (problems.length > 0) {
  console.error('check-exports: the published surface is out of sync:')
  for (const problem of problems) console.error(`  - ${problem}`)
  process.exit(1)
}

console.log(`check-exports: ${exportedPlugins.length + 1} subpaths and ${entries.length} entries agree`)
