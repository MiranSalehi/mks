import { DocumentIcon, defineResource, tr } from '../../index'
import type { PageRevision } from './types'

export const pageRevisions = defineResource<PageRevision>({
  name: 'page_revisions',
  label: tr({ fa: 'نسخهٔ برگه', en: 'Page revision' }),
  pluralLabel: tr({ fa: 'نسخه‌های برگه', en: 'Page revisions' }),
  // Opened from a page's form, as in MKSine.
  navigation: false,
  icon: <DocumentIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: '-id',
  recordTitle: (revision) => `#${revision.id}`,

  table: (t) => [
    t.text('id').label(tr({ fa: 'نسخه', en: 'Revision' })).sortable(),
    t.text('page_id').label(tr({ fa: 'شناسهٔ برگه', en: 'Page ID' })).sortable(),
    t.date('created_at').label(tr({ fa: 'ذخیره‌شده در', en: 'Saved at' })).dateTime().sortable(),
    t.text('user_id').label(tr({ fa: 'شناسهٔ کاربر', en: 'User ID' })),
  ],

  actions: (a) => [
    a.make('restore')
      .label(tr({ fa: 'بازیابی این نسخه', en: 'Restore this version' }))
      .color('warning')
      .row()
      .requiresConfirmation({
        description: tr({ fa: 'محتوای صفحه با این نسخه جایگزین می‌شود؛ نامک و وضعیت انتشار فعلی حفظ خواهد شد.', en: 'The page content will be replaced by this revision; the current slug and publication state will be preserved.' }),
        submitLabel: tr({ fa: 'بازیابی نسخه', en: 'Restore revision' }),
      }),
  ],

  infolist: (i) => [
    i.section(tr({ fa: 'اطلاعات نسخه', en: 'Revision details' })).schema([
      i.text('id').label(tr({ fa: 'شمارهٔ نسخه', en: 'Revision ID' })).numeric(),
      i.text('page_id').label(tr({ fa: 'شناسهٔ برگه', en: 'Page ID' })).numeric(),
      i.date('created_at').label(tr({ fa: 'تاریخ ذخیره', en: 'Saved at' })).dateTime(),
      i.text('user_id').label(tr({ fa: 'شناسهٔ کاربر', en: 'User ID' })),
    ]),
    i.section(tr({ fa: 'تصویر لحظه‌ای محتوا', en: 'Content snapshot' })).schema([
      i.keyValue('snapshot').label('').labels(tr({ fa: 'فیلد', en: 'Field' }), tr({ fa: 'مقدار', en: 'Value' })),
    ]),
  ],
})
