import { FolderIcon, defineResource, slugify, tr } from '../../index'
import { mediaPicker } from './media'
import { seoFields } from './seo'
import type { Category } from './types'

/** A tree of categories of any depth; the full path is the public address. */
export const categories = defineResource<Category>({
  name: 'cms_categories',
  label: tr({ fa: 'دسته', en: 'Category' }),
  pluralLabel: tr({ fa: 'دسته‌ها', en: 'Categories' }),
  navigationSort: 3,
  icon: <FolderIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: 'path_name',
  recordTitle: 'name',

  table: (t) => [
    t.image('image').label('').size(36),
    t.text('path_name').label(tr({ fa: 'دسته‌بندی', en: 'Category' })).searchable().sortable().weight('medium'),
    t.text('path').label(tr({ fa: 'نشانی', en: 'Path' })).copyable().toggleable(),
    t.text('posts_count').label(tr({ fa: 'نوشته‌ها', en: 'Posts' })).numeric(),
    t.boolean('is_active').label(tr({ fa: 'فعال', en: 'Active' })),
    t.text('sort_order').label(tr({ fa: 'ترتیب', en: 'Order' })).numeric().sortable().toggleable(true),
  ],

  filters: (f) => [
    f.ternary('is_active').label(tr({ fa: 'فعال', en: 'Active' })),
    f.ternary('parent_id').label(tr({ fa: 'سطح', en: 'Level' })).nullable()
      .labels(tr({ fa: 'زیرمجموعه', en: 'Child' }), tr({ fa: 'ریشه', en: 'Root' })),
  ],

  relations: (r) => [
    r.manager('posts').resource('cms_posts').label(tr({ fa: 'نوشته‌ها', en: 'Posts' })).table((t) => [
      t.text('title').label(tr({ fa: 'عنوان', en: 'Title' })).searchable().sortable(),
      t.badge('status').label(tr({ fa: 'وضعیت', en: 'Status' })),
    ]),
  ],

  form: (f) => [
    f.section(tr({ fa: 'دسته‌بندی', en: 'Category' })).schema([
      f.textInput('name').label(tr({ fa: 'نام', en: 'Name' })).required().maxLength(255)
        .afterStateUpdated(({ value, old, get, set, operation }) => {
          if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) set('slug', slugify(String(value ?? '')))
        }),
      f.textInput('slug').label(tr({ fa: 'نامک', en: 'Slug' })).required().dir('ltr'),
      f.select('parent_id').label(tr({ fa: 'والد', en: 'Parent' })).relationship('parent').searchable()
        .helperText(tr({ fa: 'خالی: سطح اول. خودش یا زیرمجموعه‌هایش نمی‌توانند والد باشند.', en: 'Empty: top level. It cannot sit under itself or its children.' })),
      f.textInput('sort_order').label(tr({ fa: 'ترتیب', en: 'Order' })).numeric().default(0),
      f.toggle('is_active').label(tr({ fa: 'فعال', en: 'Active' })).default(true),
      mediaPicker<Category>('image').label(tr({ fa: 'تصویر', en: 'Image' })),
      f.richEditor('description').label(tr({ fa: 'توضیح', en: 'Description' })).columnSpan('full'),
    ]),
    f.section(tr({ fa: 'سئو', en: 'SEO' })).collapsible(true).schema(seoFields(f, 'description')),
  ],
})
