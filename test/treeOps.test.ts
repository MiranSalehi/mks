import { describe, expect, it } from 'vitest'
import { indentItem, listAt, outdentItem, treeHeight } from '../src/treeOps'

const n = (label: string, children: Record<string, unknown>[] = []) => ({ label, children })
const shape = (list: any[]): unknown => list.map((x) => (x.children?.length ? [x.label, shape(x.children)] : x.label))

describe('tree moves', () => {
  const tree = [n('A'), n('B', [n('B1'), n('B2')]), n('C')]

  it('indents under the item before, as its last child', () => {
    expect(shape(indentItem(tree, [2], 'children', 6)!)).toEqual(['A', ['B', ['B1', 'B2', 'C']]])
    expect(shape(indentItem(tree, [1, 1], 'children', 6)!)).toEqual(['A', ['B', [['B1', ['B2']]]], 'C'])
    // The first item has nothing before it.
    expect(indentItem(tree, [0], 'children', 6)).toBeNull()
    expect(indentItem(tree, [1, 0], 'children', 6)).toBeNull()
  })

  it('outdents to just after the parent', () => {
    expect(shape(outdentItem(tree, [1, 0], 'children')!)).toEqual(['A', ['B', ['B2']], 'B1', 'C'])
    expect(outdentItem(tree, [1], 'children')).toBeNull()
  })

  it('keeps the depth limit, counting the item\'s own children', () => {
    expect(indentItem(tree, [1], 'children', 2)).toBeNull() // B with children would reach 3 levels
    expect(shape(indentItem(tree, [1], 'children', 3)!)).toEqual([['A', [['B', ['B1', 'B2']]]], 'C'])
    expect(treeHeight(tree[1], 'children')).toBe(2)
  })

  it('leaves the input alone and finds lists by path', () => {
    const copy = JSON.stringify(tree)
    indentItem(tree, [2], 'children', 6)
    outdentItem(tree, [1, 1], 'children')
    expect(JSON.stringify(tree)).toBe(copy)
    expect(listAt(tree, [1], 'children')?.map((x) => x.label)).toEqual(['B1', 'B2'])
    expect(listAt(tree, [9], 'children')).toBeNull()
  })
})
