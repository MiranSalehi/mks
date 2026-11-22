import { describe, expect, it } from 'vitest'
import { groupBuilder } from '../src/groups'
import { tableBuilder } from '../src/table'

type Post = { status: string; category_id: number; category?: { name: string } | null; views: number }

describe('groups', () => {
  const g = groupBuilder<Post>()
  const post: Post = { status: 'draft', category_id: 3, category: { name: 'DevOps' }, views: 5 }

  it('uses titles, a path or a function for headings', () => {
    expect(g.make('status').titles({ draft: 'پیش‌نویس' }).titleOf(post)).toBe('پیش‌نویس')
    expect(g.make('category_id').title('category.name').titleOf(post)).toBe('DevOps')
    expect(g.make('category_id').title((p) => `#${p.category_id}`).titleOf(post)).toBe('#3')
    expect(g.make('category_id').titleOf(post)).toBe('3')
    expect(g.make('category_id').title('category.name').titleOf({ ...post, category: null })).toBe('—')
  })

  it('needs the relation of its heading', () => {
    expect(g.make('category_id').title('category.name').relations).toEqual(['category'])
    expect(g.make('status').relations).toEqual([])
  })
})

describe('columns', () => {
  it('can be toggleable and summarized', () => {
    const t = tableBuilder<Post>()
    expect(t.text('views')._toggleable).toBe('no')
    expect(t.text('views').toggleable()._toggleable).toBe('shown')
    expect(t.text('views').toggleable(true)._toggleable).toBe('hidden')
    expect(t.text('views').summarize('sum', 'avg')._summaries).toEqual(['sum', 'avg'])
  })
})
