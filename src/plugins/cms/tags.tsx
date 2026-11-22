import { TagIcon, defineResource, slugify, tr } from '../../index'
import { seoFields } from './seo'
import type { Tag } from './types'

export const tags = defineResource<Tag>({
  name: 'cms_tags',
  label: tr({ fa: 'برچسب', en: 'Tag' }),
  pluralLabel: tr({ fa: 'برچسب‌ها', en: 'Tags' }),
  navigationSort: 4,
  icon: <TagIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: 'name',
  recordTitle: 'name',

  table: (t) => [
    t.text('name').label(tr({ fa: 'نام', en: 'Name' })).searchable().sortable().weight('medium'),
    t.text('slug').label(tr({ fa: 'نامک', en: 'Slug' })).searchable().copyable(),
    t.text('posts_count').label(tr({ fa: 'نوشته‌ها', en: 'Posts' })).numeric(),
    t.boolean('is_active').label(tr({ fa: 'فعال', en: 'Active' })),
    t.date('created_at').label(tr({ fa: 'ایجاد', en: 'Created' })).since().sortable().toggleable(true),
  ],

  filters: (f) => [f.ternary('is_active').label(tr({ fa: 'فعال', en: 'Active' }))],

  form: (f) => [
    f.section(tr({ fa: 'برچسب', en: 'Tag' })).schema([
      f.textInput('name').label(tr({ fa: 'نام', en: 'Name' })).required().maxLength(255)
        .afterStateUpdated(({ value, old, get, set, operation }) => {
          if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) set('slug', slugify(String(value ?? '')))
        }),
      f.textInput('slug').label(tr({ fa: 'نامک', en: 'Slug' })).dir('ltr')
        .helperText(tr({ fa: 'خالی: از نام ساخته می‌شود.', en: 'Empty: made from the name.' })),
      f.toggle('is_active').label(tr({ fa: 'فعال', en: 'Active' })).default(true),
      f.richEditor('description').label(tr({ fa: 'توضیح', en: 'Description' })).columnSpan('full'),
    ]),
    f.section(tr({ fa: 'سئو', en: 'SEO' })).collapsible(true).schema(seoFields(f, 'description')),
  ],
})
