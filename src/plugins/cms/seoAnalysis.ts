/**
 * Advisory SEO analysis, a port of MKSine's SeoAnalyzer: the same checks, thresholds and
 * scores, so editors get the same advice. Pure functions; the form shows the result live.
 */

export type Light = 'good' | 'ok' | 'bad'

export interface SeoInput {
  title?: string | null
  slug?: string | null
  metaTitle?: string | null
  metaDescription?: string | null
  keyphrase?: string | null
  /** HTML of the body. */
  content?: string | null
  /** Flesch reading ease only applies to English (as in MKSine). */
  locale?: string
}

export interface SeoCheck {
  id: string
  panel: 'seo' | 'readability'
  score: number
  light: Light
  /** For keyphrase checks without a keyphrase. */
  missing?: boolean
  /** Numbers the message may show (lengths, counts). */
  value?: number
  /** The advice to show (checks added with addSeoCheck; built-in checks have their own). */
  message?: string
}

/** What an added check sees: the input plus the body as plain text and words. */
export interface SeoCheckContext extends SeoInput {
  text: string
  words: string[]
  keyphraseNormalized: string
}

type ExtraCheck = (ctx: SeoCheckContext) => SeoCheck | SeoCheck[] | null | undefined
const extraChecks: ExtraCheck[] = []

/**
 * Adds a check to every SEO analysis (MKSine's mksine.seo.analysis_checks filter). It
 * returns one or more checks (with id, panel, score 0–100 and a message), or nothing
 * when it does not apply. Its score counts toward the overall score like the others.
 */
export function addSeoCheck(check: ExtraCheck): () => void {
  extraChecks.push(check)
  return () => {
    const i = extraChecks.indexOf(check)
    if (i >= 0) extraChecks.splice(i, 1)
  }
}

export interface SeoResult {
  overall: number
  light: Light
  checks: SeoCheck[]
  snippet: { title: string; slug: string; description: string }
}

export const lightOf = (score: number): Light => (score >= 80 ? 'good' : score >= 50 ? 'ok' : 'bad')

const entities: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ', zwnj: '‌' }

function decode(text: string): string {
  return text.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (m, e: string) => {
    if (e[0] === '#') {
      const code = e[1] === 'x' || e[1] === 'X' ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10)
      return Number.isFinite(code) ? String.fromCodePoint(code) : m
    }
    return entities[e.toLowerCase()] ?? m
  })
}

/** Lower case, entities decoded, whitespace collapsed (MKSine's SeoText). */
export function normalize(text: string): string {
  return decode(text).toLowerCase().replace(/\s+/g, ' ').trim()
}

/** The body as plain text. */
export function plainText(html: string): string {
  return normalize(html.replace(/<(script|style)[^>]*>[\s\S]*?<\/\1>/gi, ' ').replace(/<[^>]+>/g, ' '))
}

export function words(text: string): string[] {
  return text.split(/\s+/).filter((w) => /[\p{L}\p{N}]/u.test(w))
}

export function sentences(text: string): string[] {
  return text
    .split(/(?<=[.!?؟。])\s+/)
    .map((s) => s.trim())
    .filter((s) => words(s).length > 0)
}

/** The keyphrase as a slug: spaces to "-", anything but letters, digits and "-" removed. */
export function slugPart(keyphrase: string): string {
  return normalize(keyphrase).replace(/\s+/g, '-').replace(/[^\p{L}\p{N}-]/gu, '')
}

function keyphraseCheck(id: string, keyphrase: string, haystack: string): SeoCheck {
  if (!keyphrase) return { id, panel: 'seo', score: 0, light: 'bad', missing: true }
  const score = normalize(haystack).includes(keyphrase) ? 100 : 20
  return { id, panel: 'seo', score, light: lightOf(score) }
}

function band(length: number, good: [number, number], ok: [number, number]): number {
  if (length === 0) return 10
  if (length >= good[0] && length <= good[1]) return 100
  if (length >= ok[0] && length <= ok[1]) return 70
  return 30
}

function syllables(word: string): number {
  const w = word.toLowerCase().replace(/[^a-z]/g, '')
  if (!w) return 1
  const groups = w.match(/[aeiouy]+/g)?.length ?? 0
  const silentE = w.length > 2 && w.endsWith('e') && !w.endsWith('le') ? 1 : 0
  return Math.max(1, groups - silentE)
}

export function analyzeSeo(input: SeoInput): SeoResult {
  const title = input.title ?? ''
  const metaTitle = (input.metaTitle ?? '').trim() || title
  const description = input.metaDescription ?? ''
  const keyphrase = normalize(input.keyphrase ?? '')
  const html = input.content ?? ''
  const text = plainText(html)
  const bodyWords = words(text)
  const checks: SeoCheck[] = []

  // Keyphrase.
  checks.push(keyphraseCheck('keyphrase_in_title', keyphrase, title))
  checks.push(keyphraseCheck('keyphrase_in_meta_title', keyphrase, metaTitle))
  checks.push(keyphraseCheck('keyphrase_in_meta_description', keyphrase, description))
  if (!keyphrase) checks.push({ id: 'keyphrase_in_slug', panel: 'seo', score: 0, light: 'bad', missing: true })
  else {
    const part = slugPart(keyphrase)
    const score = part && normalize(input.slug ?? '').includes(part) ? 100 : 20
    checks.push({ id: 'keyphrase_in_slug', panel: 'seo', score, light: lightOf(score) })
  }
  checks.push(keyphraseCheck('keyphrase_in_intro', keyphrase, [...text].slice(0, 300).join('')))

  // Lengths and content.
  const titleLength = [...metaTitle.trim()].length
  const t = band(titleLength, [50, 60], [40, 70])
  checks.push({ id: 'meta_title_length', panel: 'seo', score: t, light: lightOf(t), value: titleLength })
  const descLength = [...description.trim()].length
  const d = band(descLength, [120, 160], [80, 180])
  checks.push({ id: 'meta_description_length', panel: 'seo', score: d, light: lightOf(d), value: descLength })
  const count = bodyWords.length
  const wc = count === 0 ? 0 : count >= 300 ? 100 : count >= 50 ? 60 : 20
  checks.push({ id: 'content_word_count', panel: 'seo', score: wc, light: lightOf(wc), value: count })

  const links = [...html.matchAll(/<a\s[^>]*href\s*=\s*["']([^"']*)["']/gi)].map((m) => m[1].trim())
  const counted = links.filter((h) => !/^(mailto|tel):/i.test(h))
  const outbound = counted.filter((h) => /^(https?:)?\/\//i.test(h))
  const internal = counted.length - outbound.length
  const li = internal > 0 ? 100 : outbound.length > 0 ? 50 : 40
  // MKSine shows "no links" as ok although its score (40) is below 50.
  checks.push({ id: 'internal_outbound_links', panel: 'seo', score: li, light: li === 100 ? 'good' : 'ok', value: internal })

  // Readability.
  const hp = /<h[1-3][\s>]/i.test(html) ? 100 : 20
  checks.push({ id: 'heading_presence', panel: 'readability', score: hp, light: lightOf(hp) })

  const paragraphs = [...html.matchAll(/<p[\s>]([\s\S]*?)<\/p>/gi)].map((m) => words(plainText(m[1])).length)
  const longest = paragraphs.length ? Math.max(...paragraphs) : 0
  const pl = paragraphs.length === 0 ? 10 : longest > 150 ? 30 : longest > 100 ? 70 : 100
  checks.push({ id: 'paragraph_length', panel: 'readability', score: pl, light: lightOf(pl), value: longest })

  const sents = sentences(text)
  let run = 0
  let maxRun = 0
  let anyLong = false
  for (const s of sents) {
    if (words(s).length > 20) {
      anyLong = true
      run++
      maxRun = Math.max(maxRun, run)
    } else run = 0
  }
  const cs = sents.length === 0 ? 10 : maxRun >= 2 ? 30 : anyLong ? 70 : 100
  checks.push({ id: 'consecutive_long_sentences', panel: 'readability', score: cs, light: lightOf(cs) })

  if ((input.locale ?? '').startsWith('en') && bodyWords.length > 0 && sents.length > 0) {
    const syl = bodyWords.reduce((n, w) => n + syllables(w), 0)
    const flesch = 206.835 - 1.015 * (bodyWords.length / sents.length) - 84.6 * (syl / bodyWords.length)
    const fr = flesch >= 60 ? 100 : flesch >= 50 ? 70 : 30
    checks.push({ id: 'flesch_reading_ease', panel: 'readability', score: fr, light: lightOf(fr), value: Math.round(flesch) })
  }

  const ctx: SeoCheckContext = { ...input, text, words: bodyWords, keyphraseNormalized: keyphrase }
  for (const extra of extraChecks) {
    try {
      const found = extra(ctx)
      for (const c of Array.isArray(found) ? found : found ? [found] : []) {
        const score = Math.max(0, Math.min(100, Math.round(c.score)))
        checks.push({ ...c, score, light: c.light ?? lightOf(score) })
      }
    } catch (e) {
      console.warn('[cms] SEO check failed', e)
    }
  }

  let overall = Math.round(checks.reduce((n, c) => n + c.score, 0) / checks.length)
  let light = lightOf(overall)
  if (text === '') {
    overall = Math.min(overall, 40)
    light = 'bad'
  }
  const snippetDescription = [...description].length > 160 ? [...description].slice(0, 157).join('') + '…' : description
  return { overall, light, checks, snippet: { title: metaTitle, slug: input.slug ?? '', description: snippetDescription } }
}
