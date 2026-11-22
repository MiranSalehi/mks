import { afterEach, describe, expect, it } from 'vitest'
import { gregorianCalendar, jalaliCalendar } from '../src/calendar'
import { direction, formatNumber, setCustomLocales, setTranslationOverrides, t, tr, useLocaleForTests } from '../src/i18n'
import en from '../src/locales/en'
import fa from '../src/locales/fa'

afterEach(() => useLocaleForTests('fa'))

describe('i18n', () => {
  it('translates with params and locale digits', () => {
    useLocaleForTests('fa')
    expect(t('list.total', { count: 1234 })).toBe('۱٬۲۳۴ رکورد')
    expect(t('common.new', { label: 'نوشته' })).toBe('نوشته جدید')
    expect(direction()).toBe('rtl')
    useLocaleForTests('en')
    expect(t('common.new', { label: 'Post' })).toBe('New Post')
    expect(formatNumber(1234)).toBe('1,234')
    expect(direction()).toBe('ltr')
  })

  it('picks English plural forms by count', () => {
    useLocaleForTests('en')
    expect(t('list.total', { count: 1 })).toBe('1 record')
    expect(t('list.total', { count: 2 })).toBe('2 records')
  })

  it('leaves unknown placeholders and has every key in both languages', () => {
    expect(t('common.searchIn')).toBe('جستجو در {name}')
    expect(Object.keys(en).sort()).toEqual(Object.keys(fa).sort())
  })

  it('picks app texts with tr()', () => {
    expect(tr({ fa: 'نوشته', en: 'Post' })).toBe('نوشته')
    useLocaleForTests('en')
    expect(tr({ fa: 'نوشته', en: 'Post' })).toBe('Post')
    expect(tr({ fa: 'فقط فارسی' })).toBe('فقط فارسی')
  })
})

describe('calendars', () => {
  it('Gregorian: month lengths, Sunday-first weeks, month steps', () => {
    expect(gregorianCalendar.monthLength(2028, 2)).toBe(29)
    expect(gregorianCalendar.monthLength(2026, 2)).toBe(28)
    expect(gregorianCalendar.firstWeekday(2026, 11)).toBe(0) // 1 Nov 2026 is a Sunday
    expect(gregorianCalendar.addMonths(2026, 12, 1)).toEqual({ y: 2027, m: 1 })
    expect(gregorianCalendar.weekdays[0]).toBe('Sun')
    expect(gregorianCalendar.monthName(11)).toBe('November')
  })

  it('Jalali through the same interface', () => {
    expect(jalaliCalendar.fromDate(new Date(2026, 10, 19))).toEqual({ y: 1405, m: 8, d: 28 })
    expect(jalaliCalendar.toGregorian({ y: 1405, m: 8, d: 28 })).toEqual({ gy: 2026, gm: 11, gd: 19 })
    expect(jalaliCalendar.addMonths(1405, 12, 1)).toEqual({ y: 1406, m: 1 })
  })
})

describe('added languages', () => {
  afterEach(() => {
    setCustomLocales([])
    setTranslationOverrides(undefined)
  })

  it('uses the base language for what is not translated', async () => {
    const { baseLocale, localeTag, messages } = await import('../src/i18n')
    setCustomLocales([{ code: 'ckb', name: 'کوردی', direction: 'rtl', base: 'en', tag: 'ckb-IQ' }])
    useLocaleForTests('ckb')
    expect(baseLocale()).toBe('en')
    expect(direction()).toBe('rtl')
    expect(localeTag()).toBe('ckb-IQ')
    expect(t('common.new', { label: 'Post' })).toBe('New Post')
    setTranslationOverrides({ 'common.save': 'پاشەکەوت' })
    expect(t('common.save')).toBe('پاشەکەوت')
    expect(tr({ fa: 'نوشته', en: 'Post' })).toBe('Post')
    expect(tr({ fa: 'نوشته', en: 'Post', ckb: 'بابەت' })).toBe('بابەت')
    expect(messages('ckb')).toBe(messages('en'))
  })

  it('falls back to a valid tag and ignores bad entries', async () => {
    const { customLocales, localeTag } = await import('../src/i18n')
    setCustomLocales([
      { code: 'ku', name: 'Kurdî', direction: 'ltr', base: 'fa', tag: 'not a tag!!' },
      { code: 'BAD', name: 'x', direction: 'ltr', base: 'en' },
      { code: 'fa', name: 'فارسی', direction: 'rtl', base: 'fa', built_in: true },
    ])
    expect(customLocales().map((l) => l.code)).toEqual(['ku'])
    useLocaleForTests('ku')
    expect(localeTag()).toBe('fa-IR')
    expect(direction()).toBe('ltr')
  })

  it('goes back to the default when the chosen language was removed', async () => {
    const { locale } = await import('../src/i18n')
    useLocaleForTests('ku')
    setCustomLocales([{ code: 'fa', name: 'فارسی', direction: 'rtl', base: 'fa', built_in: true }])
    expect(['fa', 'en']).toContain(locale())
  })
})
