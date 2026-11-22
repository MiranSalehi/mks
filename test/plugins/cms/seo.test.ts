import { describe, expect, it } from 'vitest'
import { addSeoCheck, analyzeSeo, lightOf, normalize, sentences, slugPart } from '../../../src/plugins/cms/seoAnalysis'

const check = (r: ReturnType<typeof analyzeSeo>, id: string) => r.checks.find((c) => c.id === id)!

describe('SEO analysis (MKSine thresholds)', () => {
  it('scores keyphrase placement', () => {
    const r = analyzeSeo({
      title: 'Learning Go fast',
      slug: 'learning-go-fast',
      metaDescription: 'A short guide to learning go.',
      keyphrase: 'Learning Go',
      content: '<p>Learning go is fun.</p>',
    })
    expect(check(r, 'keyphrase_in_title').score).toBe(100)
    expect(check(r, 'keyphrase_in_meta_title').score).toBe(100) // falls back to the title
    expect(check(r, 'keyphrase_in_meta_description').score).toBe(100)
    expect(check(r, 'keyphrase_in_slug').score).toBe(100)
    expect(check(r, 'keyphrase_in_intro').score).toBe(100)
    const none = analyzeSeo({ title: 'x' })
    expect(check(none, 'keyphrase_in_title')).toMatchObject({ score: 0, missing: true })
    const miss = analyzeSeo({ title: 'Other', keyphrase: 'go' })
    expect(check(miss, 'keyphrase_in_title').score).toBe(20)
  })

  it('works with Persian text', () => {
    const r = analyzeSeo({ title: 'آموزش زبان گو', slug: 'آموزش-زبان-گو', keyphrase: 'زبان گو', content: '<h2>شروع</h2><p>زبان گو ساده است؟ بله.</p>' })
    expect(check(r, 'keyphrase_in_title').score).toBe(100)
    expect(check(r, 'keyphrase_in_slug').score).toBe(100)
    expect(check(r, 'heading_presence').score).toBe(100)
    expect(sentences('زبان گو ساده است؟ بله.')).toHaveLength(2)
    expect(slugPart('  Hello, World! ')).toBe('hello-world')
    expect(normalize('A&amp;B&nbsp; C')).toBe('a&b c')
  })

  it('scores lengths with the bands', () => {
    const title = (n: number) => analyzeSeo({ title: 'x'.repeat(n) })
    expect(check(title(55), 'meta_title_length').score).toBe(100)
    expect(check(title(45), 'meta_title_length').score).toBe(70)
    expect(check(title(80), 'meta_title_length').score).toBe(30)
    const desc = (n: number) => analyzeSeo({ metaDescription: 'y'.repeat(n) })
    expect(check(desc(0), 'meta_description_length').score).toBe(10)
    expect(check(desc(140), 'meta_description_length').score).toBe(100)
    expect(check(desc(90), 'meta_description_length').score).toBe(70)
    expect(analyzeSeo({ metaDescription: 'z'.repeat(170) }).snippet.description).toHaveLength(158)
  })

  it('counts words, links, paragraphs and long sentences', () => {
    const many = (n: number) => Array.from({ length: n }, () => 'word').join(' ')
    expect(check(analyzeSeo({ content: `<p>${many(320)}</p>` }), 'content_word_count').score).toBe(100)
    expect(check(analyzeSeo({ content: `<p>${many(60)}</p>` }), 'content_word_count').score).toBe(60)
    expect(check(analyzeSeo({ content: `<p>${many(10)}</p>` }), 'content_word_count').score).toBe(20)

    expect(check(analyzeSeo({ content: '<p><a href="/about">a</a></p>' }), 'internal_outbound_links').score).toBe(100)
    expect(check(analyzeSeo({ content: '<p><a href="https://x.org">a</a> <a href="mailto:a@b.c">m</a></p>' }), 'internal_outbound_links').score).toBe(50)
    expect(check(analyzeSeo({ content: '<p>none</p>' }), 'internal_outbound_links')).toMatchObject({ score: 40, light: 'ok' })

    expect(check(analyzeSeo({ content: `<p>${many(120)}</p>` }), 'paragraph_length').score).toBe(70)
    expect(check(analyzeSeo({ content: `<p>${many(160)}</p>` }), 'paragraph_length').score).toBe(30)
    expect(check(analyzeSeo({ content: 'no paragraphs' }), 'paragraph_length').score).toBe(10)

    const long = `${many(25)}.`
    expect(check(analyzeSeo({ content: `<p>${long} short one. ${long}</p>` }), 'consecutive_long_sentences').score).toBe(70)
    expect(check(analyzeSeo({ content: `<p>${long} ${long}</p>` }), 'consecutive_long_sentences').score).toBe(30)
  })

  it('caps an empty body and adds Flesch only for English', () => {
    const empty = analyzeSeo({ title: 'x'.repeat(55), keyphrase: 'x', metaDescription: 'x'.repeat(140) })
    expect(empty.overall).toBeLessThanOrEqual(40)
    expect(empty.light).toBe('bad')
    expect(analyzeSeo({ content: '<p>The cat sat. It was happy.</p>', locale: 'en' }).checks.some((c) => c.id === 'flesch_reading_ease')).toBe(true)
    expect(analyzeSeo({ content: '<p>The cat sat.</p>', locale: 'fa' }).checks.some((c) => c.id === 'flesch_reading_ease')).toBe(false)
    expect([lightOf(80), lightOf(50), lightOf(49)]).toEqual(['good', 'ok', 'bad'])
  })
})

describe('added checks (mksine.seo.analysis_checks)', () => {
  it('counts toward the score and can be removed', () => {
    const input = { title: 'Go panels', slug: 'go-panels', keyphrase: 'go panels', content: '<p>Go panels are nice.</p>' }
    const before = analyzeSeo(input)
    const remove = addSeoCheck((ctx) =>
      ctx.text.includes('nice') ? { id: 'no_nice', panel: 'readability', score: 0, light: 'bad', message: 'Avoid "nice".' } : null,
    )
    const after = analyzeSeo(input)
    const added = after.checks.find((c) => c.id === 'no_nice')
    expect(added?.message).toBe('Avoid "nice".')
    expect(after.checks.length).toBe(before.checks.length + 1)
    expect(after.overall).toBeLessThan(before.overall)
    // A failing check is skipped, not fatal; scores are clamped.
    const removeBroken = addSeoCheck(() => {
      throw new Error('boom')
    })
    const removeLoud = addSeoCheck(() => ({ id: 'loud', panel: 'seo', score: 500, light: 'good' }))
    expect(analyzeSeo(input).checks.find((c) => c.id === 'loud')?.score).toBe(100)
    remove()
    removeBroken()
    removeLoud()
    expect(analyzeSeo(input).checks.length).toBe(before.checks.length)
  })
})
