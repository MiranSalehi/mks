import { describe, expect, it } from 'vitest'
import { CustomColumn, CustomField, defineColumn, defineField, definePlugin } from '../src/extensions'
import { initialValues } from '../src/formState'

const Noop = () => null

describe('extensions', () => {
  it('makes custom fields with their own options and empty value', () => {
    const color = defineField<string | null, { swatches: string[] }>({ component: Noop, defaults: { swatches: ['#000000'] }, empty: null })
    const a = color('color').option('swatches', ['#ffffff']).label('رنگ')
    const b = color('color')
    expect(a).toBeInstanceOf(CustomField)
    expect(a._options.swatches).toEqual(['#ffffff'])
    // Options are per field: the defaults are not shared.
    expect(b._options.swatches).toEqual(['#000000'])
    expect(initialValues([a])).toEqual({ color: null })
    expect(initialValues([a.default('#123456')])).toEqual({ color: '#123456' })
  })

  it('makes custom columns', () => {
    const swatch = defineColumn({ component: Noop, defaults: { showHex: true } })
    const c = swatch('color').option('showHex', false).label('رنگ')
    expect(c).toBeInstanceOf(CustomColumn)
    expect(c._options).toEqual({ showHex: false })
    expect(c._label).toBe('رنگ')
  })

  it('defines plugins as plain data', () => {
    const p = definePlugin({ name: 'x', resources: [], topbar: 'hi' })
    expect(p.name).toBe('x')
  })
})
