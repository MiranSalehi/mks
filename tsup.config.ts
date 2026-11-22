import { defineConfig } from 'tsup'

// One entry per public subpath. This list is the single source of truth for the build;
// scripts/check-exports.mjs fails when it drifts from "exports" in package.json or from
// src/plugins/.
export const entries = [
  'src/index.ts',
  'src/plugins/access/index.tsx',
  'src/plugins/activity/index.tsx',
  'src/plugins/appearance/index.tsx',
  'src/plugins/cms/index.tsx',
  'src/plugins/navigation/index.tsx',
  'src/plugins/system/index.tsx',
  'src/plugins/translations/index.tsx',
]

export default defineConfig({
  entry: entries,
  format: ['esm'],
  clean: true,
  sourcemap: true,
  target: 'es2022',
})
