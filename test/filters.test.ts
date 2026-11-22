import { describe, expect, it } from 'vitest'
import { activeFilters, filterBuilder, filterParams, readFilterStates, urlValue } from '../src/filters'
import { listQuery } from '../src/api'
import { validateListSearch } from '../src/router'

const f = filterBuilder<{ status: string; is_featured: boolean; published_at: string | null; created_at: string }>()

describe('filters', () => {
  it('select: eq for one value, in for several', () => {
    expect(f.select('status').params('draft')).toEqual([{ column: 'status', op: 'eq', value: 'draft' }])
    expect(f.select('status').multiple().params('draft,archived')).toEqual([{ column: 'status', op: 'in', value: ['draft', 'archived'] }])
    expect(f.select('status').multiple().params(',')).toEqual([])
  })

  it('ternary: eq, or the null operator for nullable columns', () => {
    expect(f.ternary('is_featured').params('true')).toEqual([{ column: 'is_featured', op: 'eq', value: 'true' }])
    // "yes, it has a publish date" is "published_at is not null"
    expect(f.ternary('published_at').nullable().params('true')).toEqual([{ column: 'published_at', op: 'null', value: 'false' }])
    expect(f.ternary('is_featured').params('maybe')).toEqual([])
  })

  it('date range: local days, until includes its whole day', () => {
    const params = f.dateRange('created_at').params('2026-11-01..2026-11-19')
    expect(params.map((p) => p.op)).toEqual(['gte', 'lt'])
    expect(new Date(params[0].value as string).getTime()).toBe(new Date(2026, 10, 1).getTime())
    expect(new Date(params[1].value as string).getTime()).toBe(new Date(2026, 10, 20).getTime())
    expect(f.dateRange('created_at').params('..2026-11-19').map((p) => p.op)).toEqual(['lt'])
    expect(f.dateRange('created_at').params('junk..')).toEqual([])
  })

  it('toggle sends 1', () => {
    expect(f.toggle('mine').params('1')).toEqual([{ column: 'mine', op: 'eq', value: '1' }])
    expect(f.toggle('mine').params('0')).toEqual([])
  })

  it('active filters ignore states that mean nothing', () => {
    const filters = [f.select('status'), f.dateRange('created_at'), f.toggle('mine')]
    const states = { status: 'draft', created_at: '..', mine: '1' }
    expect(activeFilters(filters, states).map((x) => x.name)).toEqual(['status', 'mine'])
    expect(filterParams(filters, states)).toHaveLength(2)
  })

  it('reads and writes the URL', () => {
    expect(readFilterStates({ 'filter[status]': 'draft,published', 'filter[mine]': 1, page: 2, 'filter[Bad!]': 'x', 'filter[x]': '' })).toEqual({
      status: 'draft,published',
      mine: '1',
    })
    expect(urlValue('1')).toBe(1)
    expect(urlValue('true')).toBe(true)
    expect(urlValue('007')).toBe('007')
    expect(urlValue('3,4')).toBe('3,4')
    expect(validateListSearch({ page: '2', 'filter[is_featured]': true, other: 'x' })).toEqual({
      page: 2,
      perPage: undefined,
      sort: undefined,
      search: undefined,
      'filter[is_featured]': true,
    })
  })

  it('builds the API query', () => {
    const q = listQuery({ filters: filterParams([f.select('status').multiple(), f.ternary('published_at').nullable()], { status: 'a,b', published_at: 'false' }) })
    expect(decodeURIComponent(q)).toBe('?filter[status][in]=a,b&filter[published_at][null]=true')
  })
})
