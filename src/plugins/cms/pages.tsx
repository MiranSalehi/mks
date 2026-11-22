import { DocumentIcon, defineResource, slugify, tr } from '../../index'
import { mediaGallery } from './media'
import { pageBuilder } from './pageBuilder'
import { seoFields } from './seo'
import type { Page, PageRevision } from './types'

const typeLabels = {
  simple: tr({ fa: 'ساده', en: 'Simple' }),
  builder: tr({ fa: 'بلوک‌محور', en: 'Block-based' }),
}

const statusLabels = {
  draft: tr({ fa: 'پیش‌نویس', en: 'Draft' }),
  published: tr({ fa: 'منتشرشده', en: 'Published' }),
  scheduled: tr({ fa: 'زمان‌بندی‌شده', en: 'Scheduled' }),
}

const widthLabels = {
  contained: tr({ fa: 'محدود', en: 'Contained' }),
  wide: tr({ fa: 'عریض', en: 'Wide' }),
  full: tr({ fa: 'تمام‌عرض', en: 'Full width' }),
}

export const pages = defineResource<Page>({
  name: 'pages',
  label: tr({ fa: 'صفحه', en: 'Page' }),
  pluralLabel: tr({ fa: 'صفحات', en: 'Pages' }),
  navigationSort: 2,
  icon: <DocumentIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: '-updated_at',
  recordTitle: 'title',

  actions: (a) => [
    a.link('trash').label(tr({ fa: 'زباله‌دان', en: 'Trash' })).url(() => '/page_trash').header(),
    a.make('preview')
      .label(tr({ fa: 'پیش‌نمایش', en: 'Preview' }))
      .color('primary')
      .row(),
    a.make('apply_template')
      .label(tr({ fa: 'گذاشتن قالب', en: 'Apply a template' }))
      .color('gray')
      .row()
      .form((f) => [
        f.radio('template').label(tr({ fa: 'قالب', en: 'Template' })).required().options({
          landing: tr({ fa: 'صفحهٔ فرود', en: 'Landing page' }),
          about: tr({ fa: 'دربارهٔ ما', en: 'About us' }),
          contact: tr({ fa: 'تماس با ما', en: 'Contact' }),
          services: tr({ fa: 'خدمات', en: 'Services' }),
          mksine_home: tr({ fa: 'صفحهٔ اصلی پوستهٔ MKSine', en: 'MKSine theme home' }),
          blank: tr({ fa: 'خالی', en: 'Blank' }),
        }),
      ])
      .modalSubmitLabel(tr({ fa: 'گذاشتن قالب', en: 'Apply' })),
    a.make('duplicate')
      .label(tr({ fa: 'ساخت کپی', en: 'Duplicate' }))
      .color('gray')
      .row()
      .requiresConfirmation({
        description: tr({ fa: 'یک برگهٔ پیش‌نویس با محتوای فعلی و نامک یکتا ساخته می‌شود.', en: 'A draft copy will be created with the current content and a unique slug.' }),
        submitLabel: tr({ fa: 'ساخت کپی', en: 'Create copy' }),
      }),
    a.make('trash')
      .label(tr({ fa: 'انتقال به زباله‌دان', en: 'Move to trash' }))
      .color('danger')
      .row()
      .requiresConfirmation({
        description: tr({ fa: 'برگه از سایت عمومی پنهان می‌شود و نامک آن برای جلوگیری از برخورد رزرو می‌ماند.', en: 'The page will be hidden from the public site. Its slug stays reserved to prevent conflicts.' }),
        submitLabel: tr({ fa: 'انتقال به زباله‌دان', en: 'Move to trash' }),
      }),
  ],

  table: (t) => [
    t.text('title').label(tr({ fa: 'عنوان', en: 'Title' })).searchable().sortable().limit(40).weight('medium'),
    t.text('slug').label(tr({ fa: 'نامک', en: 'Slug' })).searchable().copyable(),
    t.badge('type').label(tr({ fa: 'نوع صفحه', en: 'Page type' })).labels(typeLabels).toggleable(),
    t.badge('status')
      .label(tr({ fa: 'وضعیت', en: 'Status' }))
      .sortable()
      .colors({ draft: 'gray', published: 'success', scheduled: 'warning' })
      .labels(statusLabels),
    t.date('published_at').label(tr({ fa: 'زمان انتشار', en: 'Publish time' })).dateTime().sortable().toggleable(),
    t.date('created_at').label(tr({ fa: 'تاریخ ایجاد', en: 'Created at' })).since().sortable().toggleable(true),
  ],

  relations: (r) => [
    r.manager<PageRevision>('revisions')
      .resource('page_revisions')
      .label(tr({ fa: 'تاریخچهٔ نسخه‌ها', en: 'Revision history' }))
      .perPage(8)
      .table((t) => [
        t.text('id').label(tr({ fa: 'نسخه', en: 'Revision' })).sortable(),
        t.date('created_at').label(tr({ fa: 'ذخیره‌شده در', en: 'Saved at' })).dateTime().sortable(),
        t.text('user_id').label(tr({ fa: 'شناسهٔ کاربر', en: 'User ID' })),
      ]),
  ],

  filters: (f) => [
    f.select('type').label(tr({ fa: 'نوع صفحه', en: 'Page type' })).options(typeLabels),
    f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(statusLabels).multiple(),
    f.dateRange('created_at').label(tr({ fa: 'تاریخ ایجاد', en: 'Created at' })),
  ],

  form: (f) => [
    f.tabs([
      f.tab(tr({ fa: 'محتوا', en: 'Content' })).schema([
        f.textInput('title')
          .label(tr({ fa: 'عنوان', en: 'Title' }))
          .required()
          .maxLength(255)
          .afterStateUpdated(({ value, old, get, set, operation }) => {
            if (operation === 'create' && get('slug') === slugify(String(old ?? ''))) {
              set('slug', slugify(String(value ?? '')))
            }
          }),
        f.textInput('slug')
          .label(tr({ fa: 'نامک', en: 'Slug' }))
          .required()
          .dir('ltr')
          .helperText(tr({ fa: 'فقط حروف کوچک انگلیسی، عدد و خط تیره.', en: 'Lowercase English letters, numbers, and hyphens only.' })),
        f.radio('type').label(tr({ fa: 'نوع صفحه', en: 'Page type' })).options(typeLabels).inline().default('simple').required().columnSpan('full'),
        f.richEditor('content')
          .label(tr({ fa: 'متن صفحه', en: 'Page content' }))
          .columnSpan('full')
          .visible(({ get }) => get('type') === 'simple'),
        pageBuilder<Page>('builder_payload')
          .label(tr({ fa: 'بلوک‌های محتوا', en: 'Content blocks' }))
          .option('widthField', 'builder_content_width')
          .columnSpan('full')
          .visible(({ get }) => get('type') === 'builder'),
      ]),
      f.tab(tr({ fa: 'گالری', en: 'Gallery' })).schema([
        mediaGallery<Page>('gallery').label(tr({ fa: 'گالری', en: 'Gallery' })).columnSpan('full')
          .helperText(tr({ fa: 'تصویرها زیر محتوای برگه نمایش داده می‌شوند.', en: 'Shown under the page content.' })),
      ]),
      f.tab(tr({ fa: 'برچسب‌ها', en: 'Tags' })).schema([
        f.select('tags').label(tr({ fa: 'برچسب‌ها', en: 'Tags' })).relationship('tags').multiple().createOption('cms_tags').columnSpan('full'),
      ]),
      f.tab(tr({ fa: 'نمایش', en: 'Appearance' })).schema([
        f.toggle('show_page_header').label(tr({ fa: 'نمایش سربرگ صفحه', en: 'Show page header' })).default(true),
        f.select('builder_content_width')
          .label(tr({ fa: 'عرض محتوای بلوک‌ها', en: 'Block content width' }))
          .options(widthLabels)
          .default('contained')
          .visible(({ get }) => get('type') === 'builder'),
      ]),
      f.tab(tr({ fa: 'سئو', en: 'SEO' })).schema(seoFields(f)),
    ]),
    f.section(tr({ fa: 'انتشار', en: 'Publishing' })).schema([
      f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(statusLabels).default('draft').required(),
      f.dateTimePicker('published_at')
        .label(tr({ fa: 'زمان انتشار', en: 'Publish time' }))
        .required(({ get }) => get('status') === 'scheduled'),
    ]),
  ],
})
