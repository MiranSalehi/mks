import { formatNumber, locale, tr, type FieldContext, type FormBuilder } from '../../index'
import { analyzeSeo, type Light, type SeoCheck } from './seoAnalysis'

interface SEO {
  focus_keyphrase: string | null
  meta_title: string | null
  meta_description: string | null
}

const lights: Record<Light, string> = { good: 'bg-success-500', ok: 'bg-warning-500', bad: 'bg-danger-500' }
const lightNames: Record<Light, string> = {
  good: tr({ fa: 'خوب', en: 'Good' }),
  ok: tr({ fa: 'قابل قبول', en: 'OK' }),
  bad: tr({ fa: 'نیاز به کار', en: 'Needs work' }),
}

function message(c: SeoCheck): string {
  const n = formatNumber(c.value ?? 0)
  if (c.missing) return tr({ fa: 'عبارت کلیدی اصلی را وارد کنید.', en: 'Set a focus keyphrase.' })
  if (c.message) return c.message
  const ok = c.score >= 80
  switch (c.id) {
    case 'keyphrase_in_title':
      return ok ? tr({ fa: 'عبارت کلیدی در عنوان هست.', en: 'The keyphrase is in the title.' }) : tr({ fa: 'عبارت کلیدی را در عنوان بیاورید.', en: 'Use the keyphrase in the title.' })
    case 'keyphrase_in_meta_title':
      return ok ? tr({ fa: 'عبارت کلیدی در عنوان سئو هست.', en: 'The keyphrase is in the SEO title.' }) : tr({ fa: 'عبارت کلیدی را در عنوان سئو بیاورید.', en: 'Use the keyphrase in the SEO title.' })
    case 'keyphrase_in_meta_description':
      return ok ? tr({ fa: 'عبارت کلیدی در توضیحات متا هست.', en: 'The keyphrase is in the meta description.' }) : tr({ fa: 'عبارت کلیدی را در توضیحات متا بیاورید.', en: 'Use the keyphrase in the meta description.' })
    case 'keyphrase_in_slug':
      return ok ? tr({ fa: 'عبارت کلیدی در نامک هست.', en: 'The keyphrase is in the slug.' }) : tr({ fa: 'عبارت کلیدی (با خط تیره) را در نامک بیاورید.', en: 'Use the keyphrase (with hyphens) in the slug.' })
    case 'keyphrase_in_intro':
      return ok ? tr({ fa: 'عبارت کلیدی در آغاز متن هست.', en: 'The keyphrase is in the introduction.' }) : tr({ fa: 'عبارت کلیدی را در ۳۰۰ نویسهٔ اول متن بیاورید.', en: 'Use the keyphrase in the first 300 characters.' })
    case 'meta_title_length':
      return tr({ fa: `عنوان سئو ${n} نویسه است؛ بهتر: ۵۰ تا ۶۰.`, en: `The SEO title has ${n} characters; best: 50–60.` })
    case 'meta_description_length':
      return tr({ fa: `توضیحات متا ${n} نویسه است؛ بهتر: ۱۲۰ تا ۱۶۰.`, en: `The meta description has ${n} characters; best: 120–160.` })
    case 'content_word_count':
      return tr({ fa: `متن ${n} واژه دارد؛ بهتر: دست‌کم ۳۰۰.`, en: `The text has ${n} words; best: at least 300.` })
    case 'internal_outbound_links':
      return c.score === 100
        ? tr({ fa: 'به صفحه‌های خود سایت پیوند دارد.', en: 'Links to pages of this site.' })
        : c.score === 50
          ? tr({ fa: 'فقط پیوند بیرونی دارد؛ به صفحه‌های خود سایت هم پیوند دهید.', en: 'Only outbound links; link to this site too.' })
          : tr({ fa: 'پیوندی ندارد.', en: 'No links.' })
    case 'heading_presence':
      return ok ? tr({ fa: 'متن تیتر دارد.', en: 'The text has headings.' }) : tr({ fa: 'متن را با تیتر (H2، H3) بخش‌بندی کنید.', en: 'Break the text up with headings.' })
    case 'paragraph_length':
      return c.score === 10 ? tr({ fa: 'متن بند ندارد.', en: 'No paragraphs.' }) : tr({ fa: `بلندترین بند ${n} واژه است؛ بهتر: تا ۱۰۰.`, en: `The longest paragraph has ${n} words; best: up to 100.` })
    case 'consecutive_long_sentences':
      return c.score === 100
        ? tr({ fa: 'جمله‌ها کوتاه‌اند.', en: 'Sentences are short.' })
        : c.score === 10
          ? tr({ fa: 'جمله‌ای نیست.', en: 'No sentences.' })
          : tr({ fa: 'جمله‌های بالای ۲۰ واژه را کوتاه کنید، به‌ویژه پشت سر هم.', en: 'Shorten sentences over 20 words, especially in a row.' })
    case 'flesch_reading_ease':
      return tr({ fa: `خوانایی فلش ${n}؛ بهتر: ۶۰ یا بیشتر.`, en: `Flesch reading ease ${n}; best: 60 or more.` })
  }
  return c.id
}

const esc = (s: unknown) => String(s ?? '').replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[c]!)

/** The text of a block-based page as HTML, for the analysis (headings stay headings). */
function blocksToHtml(blocks: unknown): string {
  if (!Array.isArray(blocks)) return ''
  return blocks
    .map((b) => {
      const data = (b?.data ?? {}) as Record<string, unknown>
      return b?.type === 'heading' ? `<h2>${esc(data.text)}</h2>` : `<p>${esc(data.text)}</p>`
    })
    .join('')
}

/** The live SEO report of a form (MKSine's SEO panel): score, snippet and the checks. */
export function SeoReport({ get, contentField }: { get: FieldContext<any>['get']; contentField: string }) {
  const content = get('type') === 'builder' ? blocksToHtml(get('builder_payload')) : String(get(contentField) ?? '')
  const r = analyzeSeo({
    title: String(get('title') ?? get('name') ?? ''),
    slug: String(get('slug') ?? ''),
    metaTitle: String(get('meta_title') ?? ''),
    metaDescription: String(get('meta_description') ?? ''),
    keyphrase: String(get('focus_keyphrase') ?? ''),
    content,
    locale: locale(),
  })
  const panels: [SeoCheck['panel'], string][] = [
    ['seo', tr({ fa: 'سئو', en: 'SEO' })],
    ['readability', tr({ fa: 'خوانایی', en: 'Readability' })],
  ]
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <span className={`size-3 rounded-full ${lights[r.light]}`} aria-hidden="true" />
        <span className="font-semibold">{lightNames[r.light]}</span>
        <span className="text-gray-500">{tr({ fa: 'امتیاز', en: 'Score' })} {formatNumber(r.overall)} / {formatNumber(100)}</span>
      </div>
      <div className="rounded-lg p-3 ring-1 ring-gray-950/10 dark:ring-white/10" dir="auto">
        <p className="truncate text-base text-[#1a0dab] dark:text-sky-300">{r.snippet.title || tr({ fa: '(بدون عنوان)', en: '(no title)' })}</p>
        <p className="truncate text-xs text-[#006621] dark:text-emerald-400" dir="ltr">…/{r.snippet.slug}</p>
        <p className="line-clamp-2 text-xs text-gray-600 dark:text-gray-400">{r.snippet.description}</p>
      </div>
      {panels.map(([panel, label]) => (
        <div key={panel}>
          <p className="mb-1 text-xs font-semibold text-gray-500 uppercase">{label}</p>
          <ul className="space-y-1">
            {r.checks.filter((c) => c.panel === panel).map((c) => (
              <li key={c.id} className="flex items-start gap-2">
                <span className={`mt-1.5 size-2 shrink-0 rounded-full ${lights[c.light]}`} aria-label={lightNames[c.light]} />
                <span>{message(c)}</span>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  )
}

/** The SEO fields every content type shares, with the live analysis. */
export function seoFields<T extends SEO>(f: FormBuilder<T>, contentField = 'content') {
  // The names are fields of every T (it extends SEO); TypeScript cannot see that through keyof.
  const name = (n: keyof SEO) => n as Extract<keyof T, string>
  return [
    f.textInput(name('focus_keyphrase')).label(tr({ fa: 'عبارت کلیدی اصلی', en: 'Focus keyphrase' })).maxLength(191),
    f.textInput(name('meta_title')).label(tr({ fa: 'عنوان سئو', en: 'SEO title' })).maxLength(255),
    f.textarea(name('meta_description')).label(tr({ fa: 'توضیحات متا', en: 'Meta description' })).rows(3).columnSpan('full'),
    f.placeholder('seo_report')
      .label(tr({ fa: 'تحلیل سئو', en: 'SEO analysis' }))
      .content(({ get }) => <SeoReport get={get} contentField={contentField} />)
      .columnSpan('full'),
  ]
}
