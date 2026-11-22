import { describe, expect, it } from 'vitest'
import { kindOf, kindOfPath, pageList } from '../../../src/plugins/cms/mediaPickerDialog'

describe('media picker', () => {
  it('lists pages around the current one with gaps', () => {
    expect(pageList(1, 3)).toEqual([1, 2, 3])
    expect(pageList(1, 10)).toEqual([1, 2, 3, 4, '…', 10])
    expect(pageList(6, 12)).toEqual([1, '…', 5, 6, 7, '…', 12])
    expect(pageList(12, 12)).toEqual([1, '…', 9, 10, 11, 12])
  })

  it('tells file kinds by type and by path', () => {
    expect(kindOf('image/webp')).toBe('image')
    expect(kindOf('application/ogg')).toBe('audio')
    expect(kindOf('application/pdf')).toBe('document')
    expect(kindOfPath('cms/2026/a.JPG')).toBe('image')
    expect(kindOfPath('cms/b.webm')).toBe('video')
    expect(kindOfPath('cms/c.pdf')).toBe('document')
  })
})
