import { DocumentIcon, defineResource, slugify, tr } from '../../index'
import { mediaPicker } from './media'
import { postStatusLabels } from './posts'
import { seoFields } from './seo'
import type { Entry } from './types'

/** A custom content type, as registered in Go (cms.Plugin.ContentTypes). */
export interface ContentTypeDef {
  key: string
  singular: string
  plural: string
  icon?: React.ReactNode
}

/** The resource of one content type ("entries_{key}" on the server). */
export function entryResource(ct: ContentTypeDef) {
  return defineResource<Entry>({
    name: `entries_${ct.key.replaceAll('-', '_')}`,
    label: ct.singular,
    pluralLabel: ct.plural,
    icon: ct.icon ?? <DocumentIcon />,
    group: tr({ fa: 'محتوا', en: 'Content' }),
    defaultSort: '-created_at',
    recordTitle: 'title',
    table: (t) => [
      t.image('featured_image').label('').size(40),
      t.text('title').label(tr({ fa: 'عنوان', en: 'Title' })).searchable().sortable().weight('medium'),
      t.badge('status').label(tr({ fa: 'وضعیت', en: 'Status' })).sortable()
        .colors({ draft: 'gray', published: 'success', archived: 'warning' }).labels(postStatusLabels),
      t.date('published_at').label(tr({ fa: 'انتشار', en: 'Published' })).dateTime().sortable(),
    ],
    filters: (f) => [f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(postStatusLabels).multiple()],
    form: (f) => [
      f.tabs([
        f.tab(tr({ fa: 'محتوا', en: 'Content' })).schema([
          f.textInput('title').label(tr({ fa: 'عنوان', en: 'Title' })).required().maxLength(255)
            .afterStateUpdated(({ value, old, get, set, operation }) => {
              if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) set('slug', slugify(String(value ?? '')))
            }),
          f.textInput('slug').label(tr({ fa: 'نامک', en: 'Slug' })).required().dir('ltr'),
          f.textarea('excerpt').label(tr({ fa: 'خلاصه', en: 'Excerpt' })).rows(2).columnSpan('full'),
          f.richEditor('content').label(tr({ fa: 'متن', en: 'Content' })).columnSpan('full'),
        ]),
        f.tab(tr({ fa: 'سئو', en: 'SEO' })).schema(seoFields(f)),
      ]),
      f.section(tr({ fa: 'انتشار', en: 'Publishing' })).schema([
        f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(postStatusLabels).default('draft').required(),
        f.dateTimePicker('published_at').label(tr({ fa: 'زمان انتشار', en: 'Publish time' })),
        mediaPicker<Entry>('featured_image').label(tr({ fa: 'تصویر شاخص', en: 'Featured image' })),
      ]),
    ],
  })
}
