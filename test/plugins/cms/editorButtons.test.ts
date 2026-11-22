import { describe, expect, it } from 'vitest'
import { mediaHTML } from '../../../src/plugins/cms/editorButtons'
import type { Media } from '../../../src/plugins/cms/types'

const m = (over: Partial<Media>): Media => ({
  created_at: null, updated_at: null, id: 1, site_id: 1, path: 'cms/a.jpg', file_name: 'a.jpg', mime_type: 'image/jpeg',
  size: 1, width: null, height: null, alt: null, title: null, caption: null, uploaded_by: null, conversions: {}, ...over,
})

describe('insert media', () => {
  it('writes the HTML for each kind and escapes names', () => {
    expect(mediaHTML([m({ alt: 'a "b"' })])).toBe('<img src="/storage/cms/a.jpg" alt="a &quot;b&quot;">')
    expect(mediaHTML([m({ path: 'cms/v.mp4', mime_type: 'video/mp4' })])).toBe('<video src="/storage/cms/v.mp4"></video>')
    expect(mediaHTML([m({ path: 'cms/s.mp3', mime_type: 'audio/mpeg' })])).toBe('<audio src="/storage/cms/s.mp3"></audio>')
    expect(mediaHTML([m({ path: 'cms/d.pdf', mime_type: 'application/pdf', file_name: 'd<x>.pdf' })])).toBe('<p><a href="/storage/cms/d.pdf">d&lt;x></a></p>')
  })
})
