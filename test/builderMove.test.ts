import { describe, expect, it } from 'vitest'
import { moveBlock, type TreeBlock } from '../src/builderMove'

const h = (text: string): TreeBlock => ({ type: 'heading', data: { text } })
const cols = (...slots: TreeBlock[][]): TreeBlock => ({ type: 'columns', data: { layout: '1-1', children: slots.map((blocks) => ({ blocks })) } })
const names = (list: TreeBlock[]): unknown[] =>
  list.map((b) => (b.type === 'columns' ? (b.data!.children as { blocks: TreeBlock[] }[]).map((s) => names(s.blocks)) : b.data!.text))

describe('moveBlock', () => {
  it('moves within the top level', () => {
    const a = h('a'), b = h('b'), c = h('c')
    expect(names(moveBlock([a, b, c], a, { owner: null, slot: -1 }, 3)!)).toEqual(['b', 'c', 'a'])
    expect(names(moveBlock([a, b, c], c, { owner: null, slot: -1 }, 0)!)).toEqual(['c', 'a', 'b'])
  })

  it('moves into a column and back out', () => {
    const a = h('a'), x = h('x')
    const col = cols([x], [])
    const into = moveBlock([a, col], a, { owner: col, slot: 1 }, 0)!
    expect(names(into)).toEqual([[['x'], ['a']]])
    const newCol = into[0]
    const moved = (newCol.data!.children as { blocks: TreeBlock[] }[])[1].blocks[0]
    expect(names(moveBlock(into, moved, { owner: null, slot: -1 }, 1)!)).toEqual([[['x'], []], 'a'])
  })

  it('moves between columns of nested containers', () => {
    const y = h('y')
    const inner = cols([y], [])
    const outer = cols([inner], [])
    const tree = [outer]
    expect(names(moveBlock(tree, y, { owner: outer, slot: 1 }, 0)!)).toEqual([[[[[], []]], ['y']]])
  })

  it('refuses moving a container into itself', () => {
    const x = h('x')
    const col = cols([x], [])
    expect(moveBlock([col], col, { owner: col, slot: 1 }, 0)).toBeNull()
    const outer = cols([col], [])
    expect(moveBlock([outer], outer, { owner: col, slot: 0 }, 0)).toBeNull()
    expect(moveBlock([outer], h('stranger'), { owner: null, slot: -1 }, 0)).toBeNull()
    expect(moveBlock([outer, x], x, { owner: col, slot: 7 }, 0)).toBeNull()
  })

  it('keeps the tree when nothing moves and leaves other data intact', () => {
    const a = h('a'), b = h('b')
    const tree = [a, b]
    expect(moveBlock(tree, a, { owner: null, slot: -1 }, 0)).toBe(tree)
    expect(moveBlock(tree, a, { owner: null, slot: -1 }, 1)).toBe(tree)
    const col = cols([a], [b])
    ;(col.data as Record<string, unknown>).gap = 'lg'
    const out = moveBlock([col], a, { owner: col, slot: 1 }, 1)!
    expect(out[0].data!.gap).toBe('lg')
    expect(names(out)).toEqual([[[], ['b', 'a']]])
  })
})
