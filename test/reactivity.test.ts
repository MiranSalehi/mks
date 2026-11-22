import { describe, expect, it } from 'vitest'
import { formBuilder } from '../src/form'
import { applyChange, contextFor, missingRequired, payload, slugify } from '../src/formState'

const f = formBuilder<Record<string, unknown>>()

describe('slugify', () => {
  it('makes URL slugs', () => {
    expect(slugify('Hello  World!')).toBe('hello-world')
    expect(slugify('  Go & React 2026 ')).toBe('go-react-2026')
    expect(slugify('日本語 Go (12)')).toBe('go-12')
    expect(slugify('日本語')).toBe('')
  })
})

describe('applyChange', () => {
  // The Filament pattern: fill the slug from the title until the user edits the slug.
  const fields = [
    f.textInput('title').afterStateUpdated(({ value, old, get, set, operation }) => {
      if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) set('slug', slugify(String(value)))
    }),
    f.textInput('slug'),
  ]

  it('runs afterStateUpdated with set', () => {
    let values: Record<string, unknown> = { title: '', slug: '' }
    let r = applyChange(fields, values, 'title', 'Hello', 'create')
    expect(r.values).toEqual({ title: 'Hello', slug: 'hello' })
    expect(r.changed.sort()).toEqual(['slug', 'title'])

    values = r.values
    r = applyChange(fields, values, 'title', 'Hello World', 'create')
    expect(r.values.slug).toBe('hello-world')
  })

  it('stops once the slug was edited by hand', () => {
    const values = { title: 'Hello', slug: 'custom' }
    expect(applyChange(fields, values, 'title', 'Hello World', 'create').values.slug).toBe('custom')
  })

  it('does not run other fields’ hooks and leaves the input untouched', () => {
    const values = { title: 'a', slug: 'a' }
    const r = applyChange(fields, values, 'slug', 'b', 'create')
    expect(r.values).toEqual({ title: 'a', slug: 'b' })
    expect(values).toEqual({ title: 'a', slug: 'a' })
  })

  it('knows the operation', () => {
    expect(applyChange(fields, { title: '', slug: '' }, 'title', 'X', 'edit').values.slug).toBe('')
  })
})

describe('conditions', () => {
  const fields = [
    f.select('status').options({ draft: 'Draft', published: 'Published' }),
    f.textarea('body').required(({ get }) => get('status') === 'published'),
    f.toggle('is_featured').disabled(({ get }) => get('status') !== 'published'),
    f.textInput('slug').disabledOn('edit'),
    f.placeholder('words').content(({ get }) => String(String(get('body') ?? '').split(/\s+/).filter(Boolean).length)),
  ]

  it('checks conditional required fields', () => {
    expect(missingRequired(fields, { status: 'draft', body: '' }, contextFor({ status: 'draft' }, 'create'))).toEqual([])
    const values = { status: 'published', body: '  ' }
    expect(missingRequired(fields, values, contextFor(values, 'create'))).toEqual(['body'])
  })

  it('does not send disabled fields or placeholders', () => {
    const initial = { status: 'draft', body: '', is_featured: false, slug: 'a', words: '' }
    const values = { ...initial, body: 'hello there', is_featured: true, slug: 'b' }
    expect(payload('edit', fields, values, initial, contextFor(values, 'edit'))).toEqual({ body: 'hello there' })
  })

  it('sends a field again once it is enabled', () => {
    const initial = { status: 'draft', is_featured: false }
    const values = { status: 'published', is_featured: true }
    expect(payload('edit', fields, values, initial, contextFor(values, 'edit'))).toEqual({ status: 'published', is_featured: true })
  })

  it('computes placeholder content', () => {
    const words = fields[4] as ReturnType<typeof f.placeholder>
    expect(words._content(contextFor({ body: 'a b  c' }, 'create'))).toBe('3')
  })
})

import { groupErrors } from '../src/formState'

describe('groupErrors', () => {
  it('moves item errors under their field', () => {
    const fields = [f.select('tags').multiple(), f.textInput('title')]
    expect(groupErrors(fields, { 'tags.0': ['bad'], 'tags.2': ['bad'], title: ['x'], 'other.1': ['y'] })).toEqual({
      tags: ['bad'],
      title: ['x'],
      'other.1': ['y'],
    })
  })
})

import { initialValues } from '../src/formState'

describe('JSON fields', () => {
  const fields = [
    f.select('tags').relationship('tags').multiple(),
    f.repeater('faqs').schema((g) => [g.textInput('question'), g.textarea('answer')]),
    f.keyValue('meta'),
    f.tagsInput('keywords'),
    f.builder('blocks').blocks([f.block('quote').schema((g) => [g.textInput('text')])]),
  ]

  it('keeps JSON values from the record and maps only relation selects to keys', () => {
    const v = initialValues(fields, {
      tags: [{ id: 3, name: 'Go' }],
      faqs: [{ question: 'q', answer: 'a' }],
      meta: { a: '1' },
      keywords: ['x'],
      blocks: [{ type: 'quote', data: { text: 't' } }],
    })
    expect(v.tags).toEqual([3])
    expect(v.faqs).toEqual([{ question: 'q', answer: 'a' }])
    expect(v.meta).toEqual({ a: '1' })
    expect(v.keywords).toEqual(['x'])
    expect(v.blocks).toEqual([{ type: 'quote', data: { text: 't' } }])
  })

  it('starts empty with the right shapes', () => {
    expect(initialValues(fields)).toEqual({ tags: [], faqs: [], meta: {}, keywords: [], blocks: [] })
  })

  it('keeps item error paths for repeaters and builders, groups the rest', () => {
    expect(groupErrors(fields, { 'faqs.0.question': ['r'], 'blocks.1.data.text': ['r'], 'keywords.2': ['x'], 'meta.k': ['y'] })).toEqual({
      'faqs.0.question': ['r'],
      'blocks.1.data.text': ['r'],
      keywords: ['x'],
      meta: ['y'],
    })
  })

  it('treats empty lists and objects as missing for required fields', () => {
    const req = [f.keyValue('meta').required(), f.repeater('faqs').required()]
    expect(missingRequired(req, { meta: {}, faqs: [] }, contextFor({}, 'create'))).toEqual(['meta', 'faqs'])
  })
})
