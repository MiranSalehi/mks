import { describe, expect, it } from 'vitest'
import { applyMarkdown } from '../src/markdown'

describe('applyMarkdown', () => {
  it('wraps the selection and keeps it selected', () => {
    const e = applyMarkdown('hello world', 6, 11, 'bold')
    expect(e.text).toBe('hello **world**')
    expect(e.text.slice(e.start, e.end)).toBe('world')
  })

  it('inserts placeholder text without a selection', () => {
    const e = applyMarkdown('ab', 1, 1, 'code')
    expect(e.text).toBe('a`متن`b')
  })

  it('prefixes every selected line', () => {
    expect(applyMarkdown('one\ntwo\nthree', 2, 6, 'bullet').text).toBe('- one\n- two\nthree')
    expect(applyMarkdown('a\nb', 0, 3, 'number').text).toBe('1. a\n2. b')
    expect(applyMarkdown('x\ntitle', 3, 3, 'heading').text).toBe('x\n## title')
  })

  it('makes a link and selects the URL to type over', () => {
    const e = applyMarkdown('see docs', 4, 8, 'link')
    expect(e.text).toBe('see [docs](https://)')
    expect(e.text.slice(e.start, e.end)).toBe('https://')
  })
})
