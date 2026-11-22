import { baseLocale } from './i18n'
import { addJalaliMonths, firstWeekday, jalaliMonthLength, jalaliToGregorian, JALALI_MONTHS, toJalali, WEEKDAYS_SHORT } from './jalali'

/** A day in a calendar: year, month 1–12, day. */
export interface CalendarDate {
  y: number
  m: number
  d: number
}

/** What the date picker needs from a calendar. */
export interface Calendar {
  /** The local day of an instant. */
  fromDate(date: Date): CalendarDate
  toGregorian(c: CalendarDate): { gy: number; gm: number; gd: number }
  monthLength(y: number, m: number): number
  /** Column of the month's first day, in this calendar's week order. */
  firstWeekday(y: number, m: number): number
  addMonths(y: number, m: number, delta: number): { y: number; m: number }
  monthName(m: number): string
  /** Short weekday names, in week order. */
  weekdays: string[]
}

/** The Persian calendar, Saturday first. */
export const jalaliCalendar: Calendar = {
  fromDate: (date) => {
    const j = toJalali(date)
    return { y: j.jy, m: j.jm, d: j.jd }
  },
  toGregorian: (c) => jalaliToGregorian(c.y, c.m, c.d),
  monthLength: jalaliMonthLength,
  firstWeekday,
  addMonths: (y, m, delta) => {
    const r = addJalaliMonths(y, m, delta)
    return { y: r.jy, m: r.jm }
  },
  monthName: (m) => JALALI_MONTHS[m - 1],
  weekdays: WEEKDAYS_SHORT,
}

const monthNames = Array.from({ length: 12 }, (_, i) => new Intl.DateTimeFormat('en-US', { month: 'long', timeZone: 'UTC' }).format(Date.UTC(2026, i, 15)))
const weekdayNames = Array.from({ length: 7 }, (_, i) => new Intl.DateTimeFormat('en-US', { weekday: 'short', timeZone: 'UTC' }).format(Date.UTC(2026, 1, 1 + i)))

/** The Gregorian calendar, Sunday first (2026-02-01 is a Sunday). */
export const gregorianCalendar: Calendar = {
  fromDate: (date) => ({ y: date.getFullYear(), m: date.getMonth() + 1, d: date.getDate() }),
  toGregorian: (c) => ({ gy: c.y, gm: c.m, gd: c.d }),
  monthLength: (y, m) => new Date(Date.UTC(y, m, 0)).getUTCDate(),
  firstWeekday: (y, m) => new Date(Date.UTC(y, m - 1, 1)).getUTCDay(),
  addMonths: (y, m, delta) => {
    const index = y * 12 + (m - 1) + delta
    return { y: Math.floor(index / 12), m: (index % 12) + 1 }
  },
  monthName: (m) => monthNames[m - 1],
  weekdays: weekdayNames,
}

/** The calendar of the current locale: Jalali in Persian, Gregorian in English. */
export function calendar(): Calendar {
  return baseLocale() === 'fa' ? jalaliCalendar : gregorianCalendar
}
