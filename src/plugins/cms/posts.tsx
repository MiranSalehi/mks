import { DocumentIcon, defineResource, slugify, tr } from '../../index'
import { mediaGallery, mediaPicker } from './media'
import { seoFields } from './seo'
import type { Post } from './types'

export const postStatusLabels = {
  draft: tr({ fa: 'پیش‌نویس', en: 'Draft' }),
  published: tr({ fa: 'منتشرشده', en: 'Published' }),
  archived: tr({ fa: 'بایگانی', en: 'Archived' }),
}

/** Blog posts; usersResource is where authors are picked from. */
export function postsResource(usersResource = 'users') {
  return defineResource<Post>({
    name: 'cms_posts',
    label: tr({ fa: 'نوشته', en: 'Post' }),
    pluralLabel: tr({ fa: 'نوشته‌ها', en: 'Posts' }),
    navigationSort: 1,
    icon: <DocumentIcon />,
    group: tr({ fa: 'محتوا', en: 'Content' }),
    defaultSort: '-created_at',
    recordTitle: 'title',

    table: (t) => [
      t.image('featured_image').label('').size(40),
      t.text('title').label(tr({ fa: 'عنوان', en: 'Title' })).searchable().sortable().limit(50).weight('medium'),
      t.text('slug').label(tr({ fa: 'نامک', en: 'Slug' })).searchable().copyable().toggleable(true),
      t.badge('status').label(tr({ fa: 'وضعیت', en: 'Status' })).sortable()
        .colors({ draft: 'gray', published: 'success', archived: 'warning' }).labels(postStatusLabels),
      // A to-many path: the column includes the relation, format lists its names.
      t.text('categories.path_name').label(tr({ fa: 'دسته‌بندی‌ها', en: 'Categories' })).toggleable()
        .format((_, post) => (post.categories ?? []).map((c) => c.path_name).join('، ')),
      t.date('published_at').label(tr({ fa: 'انتشار', en: 'Published' })).dateTime().sortable(),
      t.text('views_count').label(tr({ fa: 'بازدید', en: 'Views' })).numeric().sortable().toggleable(true),
      t.date('created_at').label(tr({ fa: 'ایجاد', en: 'Created' })).since().sortable().toggleable(true),
    ],

    filters: (f) => [
      f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(postStatusLabels).multiple(),
      f.dateRange('published_at').label(tr({ fa: 'زمان انتشار', en: 'Publish time' })),
    ],

    form: (f) => [
      f.tabs([
        f.tab(tr({ fa: 'محتوا', en: 'Content' })).schema([
          f.textInput('title')
            .label(tr({ fa: 'عنوان', en: 'Title' }))
            .required()
            .maxLength(255)
            .afterStateUpdated(({ value, old, get, set, operation }) => {
              if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) set('slug', slugify(String(value ?? '')))
            }),
          f.textInput('slug').label(tr({ fa: 'نامک', en: 'Slug' })).required().dir('ltr')
            .helperText(tr({ fa: 'فقط حروف کوچک انگلیسی، عدد و خط تیره.', en: 'Lowercase English letters, numbers, and hyphens only.' })),
          f.textarea('excerpt').label(tr({ fa: 'خلاصه', en: 'Excerpt' })).rows(2).columnSpan('full'),
          f.richEditor('content').label(tr({ fa: 'متن', en: 'Content' })).columnSpan('full')
            .required(({ get }) => get('status') === 'published'),
          mediaGallery<Post>('gallery').label(tr({ fa: 'گالری', en: 'Gallery' })).columnSpan('full')
            .helperText(tr({ fa: 'تصویرها زیر متن نوشته نمایش داده می‌شوند.', en: 'Shown under the post.' })),
        ]),
        f.tab(tr({ fa: 'دسته‌بندی و برچسب', en: 'Categories and tags' })).schema([
          f.select('categories').label(tr({ fa: 'دسته‌بندی‌ها', en: 'Categories' })).relationship('categories').multiple().columnSpan('full')
            .helperText(tr({ fa: 'ترتیب انتخاب، ترتیب نمایش است.', en: 'The order you pick them in is the display order.' })),
          f.select('tags').label(tr({ fa: 'برچسب‌ها', en: 'Tags' })).relationship('tags').multiple().createOption('cms_tags').columnSpan('full'),
        ]),
        f.tab(tr({ fa: 'سئو', en: 'SEO' })).schema(seoFields(f)),
      ]),
      f.section(tr({ fa: 'انتشار', en: 'Publishing' })).schema([
        f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(postStatusLabels).default('draft').required(),
        f.dateTimePicker('published_at').label(tr({ fa: 'زمان انتشار', en: 'Publish time' }))
          .helperText(tr({ fa: 'زمان آینده: نوشته تا آن زمان پنهان می‌ماند.', en: 'A future time keeps the post hidden until then.' })),
        f.select('author_id').label(tr({ fa: 'نویسنده', en: 'Author' })).optionsFrom(usersResource).searchable()
          .helperText(tr({ fa: 'خالی: خودتان.', en: 'Empty: you.' })),
        mediaPicker<Post>('featured_image').label(tr({ fa: 'تصویر شاخص', en: 'Featured image' })),
      ]),
    ],
  })
}
