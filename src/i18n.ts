import en from './locales/en'
import fa from './locales/fa'

/**
 * The panel's own texts in Persian and English, plus languages a manager adds (the
 * translations plugin sends them in meta: code, name, direction and a base language whose
 * texts fill what is not translated yet). The locale is fixed for a page load: the user's
 * choice (setLocale, which reloads) or <html lang>. Definitions made at load time (a
 * column's default "yes" label) are in the right language too.
 */
export type BuiltInLocale = 'fa' | 'en'
export type Locale = BuiltInLocale | (string & {})

/** A text with plural forms (English); Persian has one form. */
export type Plural = { one: string; other: string }
export type Message = string | Plural
export type MessageKey = keyof typeof fa
export type Messages = Record<MessageKey, Message>

/** A language added by a manager (meta.extra.locales). */
export interface CustomLocale {
  code: string
  name: string
  direction: 'rtl' | 'ltr'
  base: BuiltInLocale
  /** BCP 47 tag for numbers and dates; empty: the base's. */
  tag?: string
  built_in?: boolean
}

const dictionaries: Record<BuiltInLocale, Messages> = { fa, en }
const LOCALE_KEY = 'mks-locale'
// The chosen added language, kept so the first render (before meta) uses its base.
const CUSTOM_KEY = 'mks-locale-custom'
const CODE = /^[a-z]{2,3}(_[A-Za-z0-9]{1,8})?$/

let custom: Record<string, CustomLocale> = {}

function isBuiltIn(l: string): l is BuiltInLocale {
  return l === 'fa' || l === 'en'
}

function detect(): Locale {
  try {
    const stored = localStorage.getItem(LOCALE_KEY)
    if (stored === 'fa' || stored === 'en') return stored
    if (stored && CODE.test(stored)) {
      const saved = JSON.parse(localStorage.getItem(CUSTOM_KEY) ?? 'null') as CustomLocale | null
      if (saved && saved.code === stored && isBuiltIn(saved.base)) custom[stored] = saved
      return stored
    }
  } catch {
    // no storage
  }
  const lang = typeof document !== 'undefined' ? document.documentElement.lang : ''
  return lang.startsWith('en') ? 'en' : 'fa'
}

let current: Locale = detect()

export function locale(): Locale {
  return current
}

/** The built-in language a locale falls back to (itself for fa and en). */
export function baseLocale(l: Locale = current): BuiltInLocale {
  if (isBuiltIn(l)) return l
  return custom[l]?.base ?? 'fa'
}

/**
 * The languages added by managers, from meta. When the chosen one was removed, the panel
 * goes back to the default language.
 */
export function setCustomLocales(list: CustomLocale[] | undefined) {
  const next: Record<string, CustomLocale> = {}
  for (const l of list ?? []) {
    if (!l.built_in && CODE.test(l.code) && isBuiltIn(l.base)) next[l.code] = l
  }
  custom = next
  if (!isBuiltIn(current)) {
    if (next[current]) {
      try {
        localStorage.setItem(CUSTOM_KEY, JSON.stringify(next[current]))
      } catch {
        // no storage
      }
    } else if (list) {
      // The chosen language was removed: forget it and use the default.
      try {
        localStorage.removeItem(LOCALE_KEY)
        localStorage.removeItem(CUSTOM_KEY)
      } catch {
        // no storage
      }
      current = 'fa'
    }
  }
}

/** The added languages known to this load. */
export function customLocales(): CustomLocale[] {
  return Object.values(custom)
}

export function direction(l: Locale = current): 'rtl' | 'ltr' {
  if (!isBuiltIn(l) && custom[l]) return custom[l].direction
  return baseLocale(l) === 'fa' ? 'rtl' : 'ltr'
}

function validTag(tag: string | undefined): string | undefined {
  if (!tag) return undefined
  try {
    return Intl.getCanonicalLocales(tag)[0]
  } catch {
    return undefined
  }
}

/** The BCP 47 tag for Intl: Persian uses the Persian (Jalali) calendar and digits. */
export function localeTag(l: Locale = current): string {
  if (!isBuiltIn(l)) {
    const tag = validTag(custom[l]?.tag)
    if (tag) return tag
  }
  return baseLocale(l) === 'fa' ? 'fa-IR' : 'en-US'
}

/** Switches the language: stored for this browser, then the page reloads. */
export function setLocale(l: Locale) {
  try {
    localStorage.setItem(LOCALE_KEY, l)
    if (custom[l]) localStorage.setItem(CUSTOM_KEY, JSON.stringify(custom[l]))
  } catch {
    // no storage: the switch lasts for this load only
  }
  window.location.reload()
}

/** For tests: change the locale without reloading. */
export function useLocaleForTests(l: Locale) {
  current = l
}

const numberFormats = new Map<string, Intl.NumberFormat>()

/** A cached number format for the current locale. */
export function numberFormat(options: Intl.NumberFormatOptions = {}): Intl.NumberFormat {
  const key = current + JSON.stringify(options)
  let f = numberFormats.get(key)
  if (!f) numberFormats.set(key, (f = new Intl.NumberFormat(localeTag(), options)))
  return f
}

export function formatNumber(n: number, options?: Intl.NumberFormatOptions): string {
  return numberFormat(options).format(n)
}

/** A date format for the current locale (Jalali in Persian, Gregorian in English). */
export function dateFormat(options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat(localeTag(), options)
}

/**
 * Translates a panel text. {name} placeholders take params; numbers are written in the
 * locale's digits. A Plural picks its form by params.count.
 */
export function t(key: MessageKey, params?: Record<string, string | number>): string {
  const message = overrides[key] ?? dictionaries[baseLocale()][key] ?? fa[key]
  let text: string
  if (typeof message === 'string') text = message
  else {
    const n = typeof params?.count === 'number' ? params.count : 0
    text = new Intl.PluralRules(localeTag()).select(n) === 'one' ? message.one : message.other
  }
  if (!params) return text
  return text.replace(/\{(\w+)\}/g, (whole, name: string) => {
    const v = params[name]
    if (v === undefined) return whole
    return typeof v === 'number' ? formatNumber(v) : v
  })
}

let overrides: Partial<Record<string, string>> = {}

/**
 * Replaces panel texts for this load (the translations plugin sends a manager's edits
 * with meta). Texts built before it runs (some definitions at load time) keep the default.
 */
export function setTranslationOverrides(map: Partial<Record<string, string>> | undefined) {
  overrides = map ?? {}
}

/** The built-in texts of a locale (its base's for an added one), for a translations editor. */
export function messages(l: Locale): Messages {
  return dictionaries[baseLocale(l)]
}

/**
 * Picks the app's own text for the current locale, e.g. a resource label:
 *	label: tr({ fa: 'نوشته', en: 'Post' })
 * Falls back to Persian, then English.
 */
export function tr(texts: Partial<Record<Locale, string>>): string {
  return texts[current] ?? texts[baseLocale()] ?? texts.fa ?? texts.en ?? ''
}
