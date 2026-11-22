import { describe, expect, it } from 'vitest'
import { blockAt, insertAt, listOf, pageBuilderErrorPath, pageBuilderErrorPrefix, replaceAt } from '../../../src/plugins/cms/pageBuilder'

const b = (type: string, children?: { blocks: any[] }[]) => ({ type, data: children ? { children } : {} })
const page = [b('heading'), b('columns', [{ blocks: [b('text'), b('image')] }, { blocks: [] }]), b('cta')]
const types = (list: any[] | null) => (list ?? []).map((x) => x.type)

describe('page builder paths', () => {
  it('finds lists and blocks by path', () => {
    expect(types(listOf(page, ''))).toEqual(['heading', 'columns', 'cta'])
    expect(types(listOf(page, '1.0'))).toEqual(['text', 'image'])
    expect(types(listOf(page, '1.1'))).toEqual([])
    expect(listOf(page, '1.5')).toBeNull()
    expect(blockAt(page, '1.0.1')?.type).toBe('image')
  })

  it('inserts, replaces and removes without touching the input', () => {
    const before = JSON.stringify(page)
    expect(types(listOf(insertAt(page, '1.1', 0, b('button')), '1.1'))).toEqual(['button'])
    expect(types(listOf(replaceAt(page, '1.0.0', null), '1.0'))).toEqual(['image'])
    const copied = replaceAt(page, '0', page[0], [b('heading-copy')])
    expect(types(copied)).toEqual(['heading', 'heading-copy', 'columns', 'cta'])
    expect(JSON.stringify(page)).toBe(before)
  })

  it('maps server error keys to blocks and back', () => {
    expect(pageBuilderErrorPrefix('builder_payload', '1.0.1')).toBe('builder_payload.1.data.children.0.blocks.1')
    expect(pageBuilderErrorPath('builder_payload', 'builder_payload.1.data.children.0.blocks.1.data.alt')).toEqual({ path: '1.0.1', field: 'alt' })
    expect(pageBuilderErrorPath('builder_payload', 'builder_payload.2.data.items.0.title')).toEqual({ path: '2', field: 'items.0.title' })
    expect(pageBuilderErrorPath('builder_payload', 'title')).toBeNull()
  })
})
