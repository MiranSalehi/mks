// Writes styles.css for npm: the source file with Tailwind's @source pointed at the
// built JavaScript, so consumers' Tailwind sees the package's classes.
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const root = fileURLToPath(new URL('..', import.meta.url))
const src = join(root, 'src', 'styles.css')
const out = join(root, 'styles.css')

const css = readFileSync(src, 'utf8')
if (!css.includes('@source "./"')) {
  console.error(`no @source "./" in ${src}`)
  process.exit(1)
}
writeFileSync(out, css.replaceAll('@source "./"', '@source "./dist"'))
console.log('wrote styles.css')
