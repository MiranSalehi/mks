import { describe, expect, it } from 'vitest'
import { chartLabel, defineDashboard, StatsWidget, TableWidget, widgetBuilder } from '../src/widgets'

const w = widgetBuilder()

describe('widgets', () => {
  it('builds a dashboard with defaults', () => {
    const d = defineDashboard({ widgets: (b) => [b.stats('overview'), b.chart('c'), b.table('posts')] })
    expect(d.heading).toBe('داشبورد')
    expect(d.widgets[0]).toBeInstanceOf(StatsWidget)
    // Stats and tables take the whole row; charts one column.
    expect(d.widgets.map((x) => x._columnSpan)).toEqual(['full', 1, 'full'])
    expect((d.widgets[2] as TableWidget)._resource).toBe('posts')
  })

  it('formats chart labels', () => {
    const c = w.chart('c').labels({ draft: 'Draft' })
    expect(chartLabel(c, 'draft')).toBe('Draft')
    expect(chartLabel(c, 'other')).toBe('other')
    expect(chartLabel(w.chart('m').formatLabels('month'), '2026-01')).toMatch(/ژانویه|ژانویهٔ/)
    expect(chartLabel(w.chart('d').formatLabels((l) => `#${l}`), '7')).toBe('#7')
  })

  it('describes stats', () => {
    const st = w.stats('o').stats((b) => [b.make('views').label('Views').color('info').description('Total')])._stats[0]
    expect([st.key, st._label, st._color, st._description]).toEqual(['views', 'Views', 'info', 'Total'])
    expect(w.chart('c').poll(30)._poll).toBe(30)
  })
})

import { axisTicks } from '../src/components/charts'

describe('axis ticks', () => {
  it('uses round steps and whole numbers for counts', () => {
    expect(axisTicks(25)).toEqual([0, 10, 20, 30])
    expect(axisTicks(15)).toEqual([0, 5, 10, 15])
    expect(axisTicks(4)).toEqual([0, 1, 2, 3, 4])
    expect(axisTicks(145667)).toEqual([0, 50000, 100000, 150000])
    expect(axisTicks(0)).toEqual([0, 1])
    expect(axisTicks(0.8)).toEqual([0, 0.2, 0.4, 0.6, 0.8])
  })
})
