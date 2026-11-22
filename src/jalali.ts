// Jalali (Persian) calendar helpers. Gregorian → Jalali comes from Intl (the browser's own
// calendar data); Jalali → Gregorian estimates and corrects against it. No tables, no
// dependencies, and leap years follow whatever Intl knows.

export interface JalaliDate {
  jy: number
  jm: number // 1–12
  jd: number
}

export const JALALI_MONTHS = [
  'Farvardin', 'Ordibehesht', 'Khordad', 'Tir', 'Mordad', 'Shahrivar',
  'Mehr', 'Aban', 'Azar', 'Dey', 'Bahman', 'Esfand',
]

/** Saturday first, as in Iran. */
export const WEEKDAYS_SHORT = ['Sa', 'Su', 'Mo', 'Tu', 'We', 'Th', 'Fr']

const DAY = 24 * 3600 * 1000
const formatters = new Map<string, Intl.DateTimeFormat>()

function formatter(timeZone: string | undefined): Intl.DateTimeFormat {
  const key = timeZone ?? ''
  let f = formatters.get(key)
  if (!f) {
    f = new Intl.DateTimeFormat('en-US-u-ca-persian', { year: 'numeric', month: 'numeric', day: 'numeric', timeZone })
    formatters.set(key, f)
  }
  return f
}

/** The Jalali date of an instant, in the given time zone (default: the browser's). */
export function toJalali(date: Date, timeZone?: string): JalaliDate {
  const parts = formatter(timeZone).formatToParts(date)
  const num = (type: string) => Number(parts.find((p) => p.type === type)?.value)
  return { jy: num('year'), jm: num('month'), jd: num('day') }
}

const dayOfYear = (jm: number, jd: number) => (jm <= 6 ? (jm - 1) * 31 + jd : 186 + (jm - 7) * 30 + jd)
const ordinal = (d: JalaliDate) => d.jy * 366 + dayOfYear(d.jm, d.jd)

/** The Gregorian date (year, month 1–12, day) of a Jalali date. */
export function jalaliToGregorian(jy: number, jm: number, jd: number): { gy: number; gm: number; gd: number } {
  // Start near Nowruz (20 or 21 March) plus the day of the year, then correct.
  let t = Date.UTC(jy + 621, 2, 20, 12) + (dayOfYear(jm, jd) - 1) * DAY
  const target = { jy, jm, jd }
  for (let i = 0; i < 6; i++) {
    const diff = ordinal(target) - ordinal(toJalali(new Date(t), 'UTC'))
    if (diff === 0) break
    // Across a year end the ordinal counts 366 days per year, so step at most a day there.
    t += (Math.abs(diff) > 300 ? Math.sign(diff) : diff) * DAY
  }
  const d = new Date(t)
  return { gy: d.getUTCFullYear(), gm: d.getUTCMonth() + 1, gd: d.getUTCDate() }
}

/** A leap year has 366 days: Nowruz to the next Nowruz. */
export function isLeapJalaliYear(jy: number): boolean {
  const a = jalaliToGregorian(jy, 1, 1)
  const b = jalaliToGregorian(jy + 1, 1, 1)
  return Math.round((Date.UTC(b.gy, b.gm - 1, b.gd) - Date.UTC(a.gy, a.gm - 1, a.gd)) / DAY) === 366
}

export function jalaliMonthLength(jy: number, jm: number): number {
  if (jm <= 6) return 31
  if (jm <= 11) return 30
  return isLeapJalaliYear(jy) ? 30 : 29
}

/** Column (0 = Saturday) of the first day of a Jalali month. */
export function firstWeekday(jy: number, jm: number): number {
  const g = jalaliToGregorian(jy, jm, 1)
  return (new Date(Date.UTC(g.gy, g.gm - 1, g.gd)).getUTCDay() + 1) % 7
}

export function addJalaliMonths(jy: number, jm: number, delta: number): { jy: number; jm: number } {
  const index = jy * 12 + (jm - 1) + delta
  return { jy: Math.floor(index / 12), jm: (index % 12) + 1 }
}
