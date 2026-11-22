import { describe, expect, it } from 'vitest'
import {
  addJalaliMonths,
  firstWeekday,
  isLeapJalaliYear,
  jalaliMonthLength,
  jalaliToGregorian,
  toJalali,
} from '../src/jalali'

const utc = (y: number, m: number, d: number) => new Date(Date.UTC(y, m - 1, d, 12))

describe('jalali', () => {
  it('converts Gregorian to Jalali', () => {
    expect(toJalali(utc(2026, 3, 21), 'UTC')).toEqual({ jy: 1405, jm: 1, jd: 1 })
    expect(toJalali(utc(2026, 1, 3), 'UTC')).toEqual({ jy: 1404, jm: 10, jd: 13 })
    expect(toJalali(utc(2026, 11, 19), 'UTC')).toEqual({ jy: 1405, jm: 8, jd: 28 })
  })

  it('converts Jalali to Gregorian, round-tripping every day of several years', () => {
    expect(jalaliToGregorian(1405, 1, 1)).toEqual({ gy: 2026, gm: 3, gd: 21 })
    expect(jalaliToGregorian(1403, 12, 30)).toEqual({ gy: 2025, gm: 3, gd: 20 })
    for (let t = Date.UTC(2023, 0, 1, 12); t < Date.UTC(2028, 0, 1); t += 24 * 3600 * 1000) {
      const j = toJalali(new Date(t), 'UTC')
      const g = jalaliToGregorian(j.jy, j.jm, j.jd)
      const d = new Date(t)
      expect([g.gy, g.gm, g.gd]).toEqual([d.getUTCFullYear(), d.getUTCMonth() + 1, d.getUTCDate()])
    }
  })

  it('knows leap years and month lengths', () => {
    expect(isLeapJalaliYear(1403)).toBe(true)
    expect(isLeapJalaliYear(1404)).toBe(false)
    expect(jalaliMonthLength(1403, 12)).toBe(30)
    expect(jalaliMonthLength(1404, 12)).toBe(29)
    expect(jalaliMonthLength(1405, 1)).toBe(31)
    expect(jalaliMonthLength(1405, 7)).toBe(30)
  })

  it('finds the weekday column (Saturday first)', () => {
    // 1 Farvardin 1405 = Saturday 21 March 2026
    expect(firstWeekday(1405, 1)).toBe(0)
    // 1 Azar 1405 = Sunday 22 November 2026
    expect(firstWeekday(1405, 9)).toBe(1)
    // 1 Aban 1405 = Friday 23 October 2026
    expect(firstWeekday(1405, 8)).toBe(6)
  })

  it('moves between months', () => {
    expect(addJalaliMonths(1405, 12, 1)).toEqual({ jy: 1406, jm: 1 })
    expect(addJalaliMonths(1405, 1, -1)).toEqual({ jy: 1404, jm: 12 })
  })
})

import { buildValue, parseValue } from '../src/components/DatePicker'

describe('date picker values', () => {
  it('stores date-only values as YYYY-MM-DD', () => {
    expect(buildValue({ y: 1405, m: 8, d: 28 }, false)).toBe('2026-11-19')
    expect(toJalali(parseValue('2026-11-19')!)).toEqual({ jy: 1405, jm: 8, jd: 28 })
  })

  it('round-trips a local date and time through ISO', () => {
    const iso = buildValue({ y: 1405, m: 1, d: 1 }, true, 14, 30)
    const d = parseValue(iso)!
    expect(toJalali(d)).toEqual({ jy: 1405, jm: 1, jd: 1 })
    expect([d.getHours(), d.getMinutes()]).toEqual([14, 30])
  })

  it('ignores empty or broken values', () => {
    expect(parseValue('')).toBeNull()
    expect(parseValue(null)).toBeNull()
    expect(parseValue('not a date')).toBeNull()
  })
})
