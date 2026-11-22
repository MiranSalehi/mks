import { Select, type Field, type FieldContext, type Operation } from './form'
import type { ResourceRecord } from './protocol'
import { t } from './i18n'

export const REQUIRED_MESSAGE = t('common.required')

/** The context fields see for the current values. */
export function contextFor(values: ResourceRecord, operation: Operation, record?: ResourceRecord): FieldContext<any> {
  return { get: (name) => values[name], operation, record }
}

/**
 * Applies a user's change to one field and runs its afterStateUpdated hook.
 * Returns the new values and every field that changed (the field itself and any set()).
 */
export function applyChange(
  fields: Field<any>[],
  values: ResourceRecord,
  name: string,
  value: unknown,
  operation: Operation,
  record?: ResourceRecord,
): { values: ResourceRecord; changed: string[] } {
  const next: ResourceRecord = { ...values, [name]: value }
  const changed = new Set([name])
  const field = fields.find((f) => f.name === name)
  if (field?._afterStateUpdated) {
    field._afterStateUpdated({
      ...contextFor(next, operation, record),
      value,
      old: values[name],
      set: (other, v) => {
        next[other] = v
        changed.add(other)
      },
    })
  }
  return { values: next, changed: [...changed] }
}

/**
 * Puts errors for parts of a field (e.g. "tags.0" for one item of a multiple select) under
 * the field itself, so they show where the user can fix them.
 */
export function groupErrors(fields: Field<any>[], errors: Record<string, string[]>): Record<string, string[]> {
  const names = new Set(fields.map((f) => f.name as string))
  // Repeaters and builders show errors on the fields inside their items, so keep those paths.
  const nested = new Set(fields.filter((f) => f.kind === 'repeater' || f.kind === 'builder').map((f) => f.name as string))
  const out: Record<string, string[]> = {}
  for (const [key, msgs] of Object.entries(errors)) {
    const base = key.split('.')[0]
    const target = names.has(key) || nested.has(base) ? key : names.has(base) ? base : key
    out[target] = [...new Set([...(out[target] ?? []), ...msgs])]
  }
  return out
}

/** Required fields that are empty, checked in the browser before submitting. */
export function missingRequired(visible: Field<any>[], values: ResourceRecord, ctx: FieldContext<any>): string[] {
  return visible
    .filter((f) => f.isRequired(ctx) && !f.isDisabled(ctx) && f.kind !== 'hidden' && f.kind !== 'placeholder')
    .filter((f) => {
      const v = normalize(values[f.name])
      const emptyObject = typeof v === 'object' && v !== null && !Array.isArray(v) && Object.keys(v).length === 0
      return v === null || (typeof v === 'string' && v.trim() === '') || (Array.isArray(v) && v.length === 0) || emptyObject
    })
    .map((f) => f.name)
}

/** Turns "Hello World 2!" into "hello-world-2". Persian digits become Latin; other letters are dropped. */
export function slugify(text: string): string {
  return text
    .replace(/[۰-۹]/g, (d) => String('۰۱۲۳۴۵۶۷۸۹'.indexOf(d)))
    .normalize('NFKD')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

/** Empty strings become null, like Laravel's ConvertEmptyStringsToNull. */
export function normalize(v: unknown): unknown {
  return v === '' || v === undefined ? null : v
}

export function sameValue(a: unknown, b: unknown): boolean {
  return JSON.stringify(normalize(a)) === JSON.stringify(normalize(b))
}

/** Starting values: from the record when editing, otherwise each field's default. */
export function initialValues(fields: Field<any>[], record?: ResourceRecord): ResourceRecord {
  const values: ResourceRecord = {}
  for (const f of fields) {
    if (record) values[f.name] = fromRecord(f, record[f.name])
    else values[f.name] = f._default !== undefined ? f._default : f.emptyValue
  }
  return values
}

// To-many relations arrive as objects (with include); a multiple relation select holds
// their keys. Everything else (repeaters, tags, key-value) is kept as it is.
function fromRecord(f: Field<any>, v: unknown): unknown {
  if (v == null) return f.emptyValue
  if (f instanceof Select && f._multiple && f._relationship && Array.isArray(v)) {
    return v.map((item) => (item !== null && typeof item === 'object' ? (item as ResourceRecord).id : item))
  }
  return v
}

/** Names of the fields whose value differs from the starting value. */
export function dirtyFields(fields: Field<any>[], values: ResourceRecord, initial: ResourceRecord): string[] {
  return fields
    .filter((f) => f.kind !== 'placeholder' && !sameValue(values[f.name], initial[f.name]))
    .map((f) => f.name)
}

/**
 * What to send to the server.
 *  - create: every visible field that has a value (empty ones are left out, so database
 *    defaults apply), and booleans always.
 *  - edit: only the visible fields that changed (PATCH is partial); cleared fields send null.
 * Fields that are not dehydrated (disabled by default, placeholders) are never sent.
 */
export function payload(
  mode: 'create' | 'edit',
  visible: Field<any>[],
  values: ResourceRecord,
  initial: ResourceRecord,
  ctx: FieldContext<any> = contextFor(values, mode),
): ResourceRecord {
  const data: ResourceRecord = {}
  for (const f of visible) {
    if (!f.isDehydrated(ctx)) continue
    const v = normalize(values[f.name])
    if (mode === 'edit') {
      if (!sameValue(values[f.name], initial[f.name])) data[f.name] = v
    } else if (v !== null || typeof f.emptyValue === 'boolean') {
      data[f.name] = v ?? f.emptyValue
    }
  }
  return data
}
