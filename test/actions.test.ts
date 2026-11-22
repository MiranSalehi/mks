import { describe, expect, it } from 'vitest'
import { actionBuilder } from '../src/actions'

const a = actionBuilder<{ status: string }>()

describe('actions', () => {
  it('delete asks first and shows on rows and bulk', () => {
    const del = a.delete()
    expect(del.kind).toBe('delete')
    expect(del.placements.sort()).toEqual(['bulk', 'row'])
    expect(del.needsModal).toBe(true)
    expect(del._color).toBe('danger')
  })

  it('server actions are placed where asked and run directly unless confirmed or with a form', () => {
    const publish = a.make('publish').row().bulk()
    expect(publish.placements.sort()).toEqual(['bulk', 'row'])
    expect(publish.needsModal).toBe(false)
    expect(a.make('x').requiresConfirmation().needsModal).toBe(true)
    expect(a.make('y').form((f) => [f.textInput('reason')]).needsModal).toBe(true)
    expect(a.make('z').header().placements).toEqual(['header'])
  })

  it('row visibility depends on the record', () => {
    const publish = a.make('publish').row().visible((r) => r.status !== 'published')
    expect(publish.isVisibleFor({ status: 'draft' })).toBe(true)
    expect(publish.isVisibleFor({ status: 'published' })).toBe(false)
  })

  it('link actions build URLs per record', () => {
    const view = a.link('view').url((r) => `/blog/${r.status}`, true)
    expect(view._url({ status: 'x' })).toBe('/blog/x')
    expect(view._newTab).toBe(true)
    expect(view.placements).toEqual(['row'])
  })
})
