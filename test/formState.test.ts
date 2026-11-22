import { describe, expect, it } from 'vitest'
import { formBuilder } from '../src/form'
import { dirtyFields, initialValues, payload, sameValue } from '../src/formState'

const f = formBuilder<Record<string, unknown>>()
const fields = [
  f.textInput('title'),
  f.select('type').options({ post: 'Post', page: 'Page' }).default('post'),
  f.textarea('excerpt'),
  f.toggle('is_featured'),
  f.select('category_id'),
]

describe('initialValues', () => {
  it('uses defaults and empty values for a new record', () => {
    expect(initialValues(fields)).toEqual({ title: '', type: 'post', excerpt: '', is_featured: false, category_id: '' })
  })

  it('uses the record when editing, keeping types', () => {
    const v = initialValues(fields, { title: 'Hi', type: 'page', excerpt: null, is_featured: true, category_id: 3, views: 9 })
    expect(v).toEqual({ title: 'Hi', type: 'page', excerpt: '', is_featured: true, category_id: 3 })
  })
})

describe('sameValue', () => {
  it('treats empty string and null as the same', () => {
    expect(sameValue('', null)).toBe(true)
    expect(sameValue(3, 3)).toBe(true)
    expect(sameValue(3, '3')).toBe(false)
  })
})

describe('payload', () => {
  it('create: sends filled fields and booleans, leaves empty ones out', () => {
    const values = { title: 'New', type: 'post', excerpt: '', is_featured: false, category_id: 2 }
    expect(payload('create', fields, values, initialValues(fields))).toEqual({
      title: 'New',
      type: 'post',
      is_featured: false,
      category_id: 2,
    })
  })

  it('edit: sends only changed fields, cleared ones as null', () => {
    const initial = initialValues(fields, { title: 'Old', type: 'post', excerpt: 'x', is_featured: false, category_id: 1 })
    const values = { ...initial, title: 'New', excerpt: '' }
    expect(payload('edit', fields, values, initial)).toEqual({ title: 'New', excerpt: null })
    expect(dirtyFields(fields, values, initial)).toEqual(['title', 'excerpt'])
  })

  it('edit: hidden fields are not sent even if changed', () => {
    const initial = initialValues(fields, { title: 'Old', type: 'post', excerpt: 'x' })
    const values = { ...initial, excerpt: 'changed' }
    const visible = fields.filter((x) => x.name !== 'excerpt')
    expect(payload('edit', visible, values, initial)).toEqual({})
  })
})
