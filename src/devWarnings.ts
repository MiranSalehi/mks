import type { MetaResponse } from './protocol'
import type { ResourceDefinition } from './resource'

const isDev = (() => {
  try {
    return Boolean((import.meta as unknown as { env?: { DEV?: boolean } }).env?.DEV)
  } catch {
    return false
  }
})()

const warned = new Set<string>()

/**
 * In development, compares the frontend definitions with what the server allows, so a
 * sortable() column the server does not whitelist shows up as a warning instead of a 400.
 */
export function checkDefinitions(definitions: ResourceDefinition[], meta: MetaResponse, pages: { name: string }[] = []) {
  if (!isDev) return
  const warn = (msg: string) => {
    if (warned.has(msg)) return
    warned.add(msg)
    console.warn(`[panel] ${msg}`)
  }

  for (const page of pages) {
    if (definitions.some((d) => d.name === page.name)) {
      warn(`page "${page.name}" has the name of a resource; pages and resources share the URL /{name}.`)
    }
  }

  for (const def of definitions) {
    const server = meta.resources[def.name]
    if (!server) {
      warn(`resource "${def.name}" is not registered on the server (or the user cannot see it); it is hidden.`)
      continue
    }
    const caps = server.capabilities
    for (const col of def.columns) {
      if (col._sortable && !caps.sortable.includes(col.name)) {
        warn(`${def.name}: column "${col.name}" is sortable() but the server does not allow sorting by it.`)
      }
      if (col._searchable && caps.searchable.length === 0) {
        warn(`${def.name}: column "${col.name}" is searchable() but the server has no searchable fields.`)
      }
      for (const fn of col._summaries) {
        if (!(caps.summaries?.[col.name] ?? []).includes(fn)) {
          warn(`${def.name}: column "${col.name}" summarizes "${fn}", which the server does not allow (Summaries).`)
        }
      }
      for (const rel of col.relations) {
        if (!server.relations.includes(rel)) {
          warn(`${def.name}: column "${col.name}" needs relation "${rel}", which the server does not offer.`)
        }
      }
    }
    for (const field of def.fields) {
      // Display-only fields are never sent.
      if (field.kind === 'placeholder' || field._dehydrated === false) continue
      if (!caps.fillable.includes(field.name)) {
        warn(`${def.name}: form field "${field.name}" is not fillable on the server; saving will fail.`)
      }
    }
    for (const action of def.actions) {
      if (action.kind === 'delete' && !server.deletable) {
        warn(`${def.name}: has a delete action but the server resource is not Deletable.`)
      }
      if (action.kind === 'server') {
        const sa = server.actions.find((a) => a.name === action.name)
        if (!sa) {
          warn(`${def.name}: action "${action.name}" is not defined on the server.`)
        } else if (sa.without_records !== action._placements.has('header')) {
          warn(`${def.name}: action "${action.name}": header() must match WithoutRecords on the server.`)
        }
      }
    }
    for (const filter of def.filters) {
      const custom = (caps.filters ?? []).includes(filter.name)
      const allowed = caps.filterable[filter.name] ?? []
      if (!custom && !filter.operators.every((op) => allowed.includes(op))) {
        warn(`${def.name}: filter "${filter.name}" needs ${filter.operators.join(', ')}, which the server does not allow (Filterable or a custom filter).`)
      }
      if (filter.kind === 'select' && (filter as { _relationship?: string })._relationship) {
        const rel = (filter as { _relationship?: string })._relationship!
        if (!server.relations.includes(rel)) warn(`${def.name}: filter "${filter.name}" uses relation "${rel}", which the server does not offer.`)
      }
    }
    for (const g of def.groups) {
      if (!caps.sortable.includes(g.name)) {
        warn(`${def.name}: group "${g.name}" needs sorting by "${g.name}", which the server does not allow.`)
      }
    }
    for (const m of def.relations) {
      if (!server.relations.includes(m.relation)) {
        warn(`${def.name}: relation manager "${m.relation}" needs a relation the server does not offer (Relations in Config).`)
      }
      if (!definitions.some((d) => d.name === m._resource)) {
        warn(`${def.name}: relation manager "${m.relation}" uses resource "${m._resource}", which has no frontend definition.`)
      }
    }
    const sortCol = def.defaultSort?.replace(/^-/, '')
    if (sortCol && !caps.sortable.includes(sortCol) && sortCol !== caps.reorderable) {
      warn(`${def.name}: defaultSort "${def.defaultSort}" is not sortable on the server.`)
    }
  }
}
