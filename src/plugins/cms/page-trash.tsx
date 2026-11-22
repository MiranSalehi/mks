import { DocumentIcon, defineResource, tr } from '../../index'
import type { PageTrash } from './types'

export const pageTrash = defineResource<PageTrash>({
  name: 'page_trash',
  label: tr({ fa: 'برگهٔ حذف‌شده', en: 'Trashed page' }),
  pluralLabel: tr({ fa: 'زباله‌دان برگه‌ها', en: 'Page trash' }),
  // Opened from the pages list ("Trash").
  navigation: false,
  icon: <DocumentIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: '-deleted_at',
  recordTitle: 'title',

  table: (t) => [
    t.text('title').label(tr({ fa: 'عنوان', en: 'Title' })).searchable().sortable().weight('medium'),
    t.text('slug').label(tr({ fa: 'نامک', en: 'Slug' })).searchable().copyable(),
    t.badge('status')
      .label(tr({ fa: 'وضعیت پیشین', en: 'Previous status' }))
      .colors({ draft: 'gray', published: 'success', scheduled: 'warning' })
      .labels({
        draft: tr({ fa: 'پیش‌نویس', en: 'Draft' }),
        published: tr({ fa: 'منتشرشده', en: 'Published' }),
        scheduled: tr({ fa: 'زمان‌بندی‌شده', en: 'Scheduled' }),
      }),
    t.date('deleted_at').label(tr({ fa: 'زمان حذف', en: 'Deleted at' })).dateTime().sortable(),
  ],

  actions: (a) => [
    a.make('restore')
      .label(tr({ fa: 'بازیابی', en: 'Restore' }))
      .color('success')
      .row()
      .requiresConfirmation({
        description: tr({ fa: 'برگه به فهرست برمی‌گردد و وضعیت و زمان انتشار قبلی حفظ می‌شود. اگر طبق آن تنظیمات انتشار رسیده باشد، برگه بلافاصله در سایت عمومی نمایش داده خواهد شد.', en: 'The page returns to the list with its previous status and publication date. If it is due for publication, it will immediately appear on the public site.' }),
        submitLabel: tr({ fa: 'بازیابی برگه', en: 'Restore page' }),
      }),
    a.make('purge')
      .label(tr({ fa: 'حذف دائمی', en: 'Delete permanently' }))
      .color('danger')
      .row()
      .requiresConfirmation({
        description: tr({ fa: 'این عملیات برگشت‌پذیر نیست و محتوای برگه برای همیشه حذف می‌شود.', en: 'This cannot be undone. The page and its history will be deleted permanently.' }),
        submitLabel: tr({ fa: 'حذف دائمی', en: 'Delete permanently' }),
      }),
  ],

  infolist: (i) => [
    i.section(tr({ fa: 'برگهٔ حذف‌شده', en: 'Trashed page' })).schema([
      i.text('title').label(tr({ fa: 'عنوان', en: 'Title' })).weight('semibold'),
      i.text('slug').label(tr({ fa: 'نامک', en: 'Slug' })).copyable(),
      i.badge('status').label(tr({ fa: 'وضعیت پیشین', en: 'Previous status' })).colors({ draft: 'gray', published: 'success', scheduled: 'warning' }),
      i.date('deleted_at').label(tr({ fa: 'زمان حذف', en: 'Deleted at' })).dateTime(),
    ]),
  ],
})
