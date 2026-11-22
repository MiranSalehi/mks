import { describe, expect, it } from 'vitest'
import { entryPaths, infolistBuilder, InfoSection, RepeatableEntry } from '../src/infolist'

type FAQ = { question: string; answer: string }
type Post = { title: string; status: string; keywords: string[]; tags?: { id: number; name: string }[]; faqs: FAQ[]; author?: { name: string } }

const i = infolistBuilder<Post>()

describe('infolist', () => {
  it('lists strings or a field of related records', () => {
    const post = { keywords: ['go', '', 'react'], tags: [{ id: 1, name: 'Go' }, { id: 2, name: 'React' }] }
    expect(i.list('keywords').items(post)).toEqual(['go', 'react'])
    expect(i.list('tags').title('name').items(post)).toEqual(['Go', 'React'])
    expect(i.list('tags').items({})).toEqual([])
  })

  it('collects entry paths through layouts, for includes', () => {
    const nodes = [
      i.section('a').schema([i.text('title'), i.text('author.name')]),
      i.grid(2).schema([i.list('tags').title('name')]),
      i.repeatable('faqs').schema((q) => [q.text('question')]),
    ]
    expect(entryPaths(nodes)).toEqual(['title', 'author.name', 'tags', 'faqs'])
  })

  it('builds sections, spans and conditions fluently', () => {
    const s = i.section('Status').aside().columns(1).description('d')
    expect(s).toBeInstanceOf(InfoSection)
    expect([s._aside, s._columns, s._description]).toEqual([true, 1, 'd'])
    const date = i.date('title').columnSpan('full').visible((p) => p.status === 'published')
    expect(date._columnSpan).toBe('full')
    expect(date._visible?.({ status: 'draft' } as Post)).toBe(false)
  })

  it('types repeatable items by the field', () => {
    const r = i.repeatable('faqs').schema((q) => [q.text('question'), q.text('answer')])
    expect(r).toBeInstanceOf(RepeatableEntry)
    expect(r._schema.map((n) => ('name' in n ? n.name : ''))).toEqual(['question', 'answer'])
  })
})
