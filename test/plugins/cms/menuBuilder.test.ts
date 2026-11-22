import { describe, expect, it } from 'vitest'
import { indent, outdent, toRows, toTree } from '../../../src/plugins/cms/menuBuilder'
import type { MenuItem } from '../../../src/plugins/cms/types'

const link = (label: string, children: MenuItem[] = []): MenuItem =>
  ({ type: 'link', label, url: '/' + label, new_tab: false, page_id: null, post_id: null, category_id: null, tag_id: null, children }) as MenuItem
const shape = (tree: MenuItem[]): unknown => tree.map((n) => (n.children.length ? [n.label, shape(n.children)] : n.label))
const depths = (rows: ReturnType<typeof toRows>) => rows.map((r) => `${r.item.label}${r.depth}`).join(' ')

describe('menu builder rows', () => {
  const tree = [link('A'), link('B', [link('C'), link('D', [link('E')])]), link('F')]

  it('flattens in order with depths and builds the same tree back', () => {
    const rows = toRows(tree)
    expect(depths(rows)).toBe('A0 B0 C1 D1 E2 F0')
    expect(shape(toTree(rows))).toEqual(shape(tree))
  })

  it('nests under the row above, with its children', () => {
    const rows = toRows(tree)
    expect(depths(indent(rows, 1)!)).toBe('A0 B1 C2 D2 E3 F0')
    expect(depths(indent(rows, 3)!)).toBe('A0 B0 C1 D2 E3 F0')
    expect(indent(rows, 0)).toBeNull() // nothing above
    expect(indent(rows, 2)).toBeNull() // already the first child
  })

  it('moves out to after the parent and its other children', () => {
    const rows = toRows(tree)
    expect(shape(toTree(outdent(rows, 2)!))).toEqual(['A', ['B', [['D', ['E']]]], 'C', 'F'])
    expect(shape(toTree(outdent(rows, 4)!))).toEqual(['A', ['B', ['C', 'D', 'E']], 'F'])
    expect(outdent(rows, 0)).toBeNull()
  })
})
