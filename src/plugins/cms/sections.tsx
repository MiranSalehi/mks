import { SquaresIcon, defineResource, tr } from '../../index'
import { pageBuilder } from './pageBuilder'
import type { Section } from './types'

/** Reusable groups of blocks; a "Saved section" block shows one in any page. */
export const sections = defineResource<Section>({
  name: 'cms_sections',
  label: tr({ fa: 'سکشن ذخیره‌شده', en: 'Saved section' }),
  pluralLabel: tr({ fa: 'سکشن‌های ذخیره‌شده', en: 'Saved sections' }),
  navigationSort: 3,
  icon: <SquaresIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: 'name',
  recordTitle: 'name',
  table: (t) => [
    t.text('name').label(tr({ fa: 'نام', en: 'Name' })).searchable().sortable().weight('medium'),
    t.text('builder_payload').label(tr({ fa: 'بلوک‌ها', en: 'Blocks' })).numeric().format((v) => String(Array.isArray(v) ? v.length : 0)),
    t.date('updated_at').label(tr({ fa: 'آخرین تغییر', en: 'Last changed' })).since().sortable(),
  ],
  form: (f) => [
    f.textInput('name').label(tr({ fa: 'نام', en: 'Name' })).required().maxLength(255),
    pageBuilder<Section>('builder_payload')
      .label(tr({ fa: 'بلوک‌ها', en: 'Blocks' }))
      .helperText(tr({ fa: 'تغییر سکشن، همهٔ صفحه‌هایی را که از آن استفاده می‌کنند عوض می‌کند.', en: 'Changing a section changes every page that uses it.' }))
      .columnSpan('full'),
  ],
})
