import { describe, expect, it } from 'vitest'
import { ApiClient } from '../src/api'
import { formBuilder } from '../src/form'

function recordingClient() {
  const api = new ApiClient('/admin/api')
  const paths: string[] = []
  // Only the path matters here; request() is the one place that talks to the server.
  ;(api as unknown as { request: (path: string) => Promise<unknown> }).request = (path) => {
    paths.push(path)
    return Promise.resolve({ data: [], meta: { page: 1, has_more: false } })
  }
  return { api, paths }
}

describe('select options', () => {
  it('reads a relation of the resource, or the records of a resource itself', async () => {
    const { api, paths } = recordingClient()
    await api.options('posts', 'category', { search: 'go' })
    await api.options('pages', undefined, { search: 'about us', page: 2 })
    await api.options('pages', undefined, { values: [3, 7] })
    await api.options('pages', undefined)
    expect(paths).toEqual([
      '/resources/posts/relations/category/options?search=go',
      '/resources/pages/options?search=about+us&page=2',
      '/resources/pages/options?values=3%2C7',
      '/resources/pages/options',
    ])
  })

  it('optionsFrom names the resource whose records are the options', () => {
    const f = formBuilder<Record<string, unknown>>()
    const select = f.select('page_id').optionsFrom('pages').searchable()
    expect(select._optionsFrom).toBe('pages')
    expect(select._relationship).toBeUndefined()
    expect(select.emptyValue).toBe('')
  })
})
