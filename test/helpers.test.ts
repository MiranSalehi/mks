import { describe, expect, it } from 'vitest'
import { listQuery } from '../src/api'
import { relativeTime } from '../src/components/cells'
import { formatSort, pageItems, parseSort } from '../src/components/ResourceList'
import { validateListSearch } from '../src/router'
import { TextColumn, readPath } from '../src/table'

describe('sort param', () => {
  it('round-trips multi-column sorting', () => {
    const state = parseSort('-views,title')
    expect(state).toEqual([
      { id: 'views', desc: true },
      { id: 'title', desc: false },
    ])
    expect(formatSort(state)).toBe('-views,title')
  })

  it('treats empty as no sorting', () => {
    expect(parseSort(undefined)).toEqual([])
    expect(formatSort([])).toBeUndefined()
  })
})

describe('pageItems', () => {
  it('shows all pages when there are few', () => {
    expect(pageItems(2, 3)).toEqual([1, 2, 3])
  })

  it('keeps first, last and neighbours with gaps', () => {
    expect(pageItems(5, 10)).toEqual([1, 'gap', 4, 5, 6, 'gap', 10])
    expect(pageItems(1, 10)).toEqual([1, 2, 'gap', 10])
    expect(pageItems(10, 10)).toEqual([1, 'gap', 9, 10])
  })
})

describe('validateListSearch', () => {
  it('keeps valid values and drops junk', () => {
    expect(validateListSearch({ page: '3', perPage: 25, sort: '-title', search: 'go' })).toEqual({
      page: 3,
      perPage: 25,
      sort: '-title',
      search: 'go',
    })
    expect(validateListSearch({ page: '-1', perPage: 'abc', sort: '', search: '   ' })).toEqual({
      page: undefined,
      perPage: undefined,
      sort: undefined,
      search: undefined,
    })
  })

  it('keeps numeric-looking search text', () => {
    // The router may parse "?search=2026" as a number.
    expect(validateListSearch({ search: 2026 }).search).toBe('2026')
  })
})

describe('listQuery', () => {
  it('builds the protocol query string', () => {
    const q = listQuery({
      page: 2,
      perPage: 25,
      sort: ['-views', 'title'],
      search: 'go & react',
      include: ['category'],
      filters: [
        { column: 'status', op: 'in', value: ['draft', 'published'] },
        { column: 'type', op: 'eq', value: 'post' },
      ],
    })
    const params = new URLSearchParams(q.slice(1))
    expect(params.get('page')).toBe('2')
    expect(params.get('per_page')).toBe('25')
    expect(params.get('sort')).toBe('-views,title')
    expect(params.get('search')).toBe('go & react')
    expect(params.get('include')).toBe('category')
    expect(params.get('filter[status][in]')).toBe('draft,published')
    expect(params.get('filter[type]')).toBe('post')
  })

  it('is empty without params', () => {
    expect(listQuery({})).toBe('')
  })
})

describe('columns', () => {
  it('reads nested paths and collects relations', () => {
    const col = new TextColumn('category.name').description('author.name')
    expect(col.relations.sort()).toEqual(['author', 'category'])
    expect(col.value({ category: { name: 'Go' } })).toBe('Go')
    expect(readPath({ a: null }, 'a.b')).toBeUndefined()
  })
})

describe('relativeTime', () => {
  const now = Date.UTC(2026, 10, 19, 12, 0, 0)
  it('picks the largest fitting unit', () => {
    expect(relativeTime(new Date(now - 3 * 24 * 3600 * 1000), now)).toBe(new Intl.RelativeTimeFormat('fa-IR', { numeric: 'auto' }).format(-3, 'day'))
    expect(relativeTime(new Date(now - 90 * 60 * 1000), now)).toBe(new Intl.RelativeTimeFormat('fa-IR', { numeric: 'auto' }).format(-2, 'hour'))
  })
})
