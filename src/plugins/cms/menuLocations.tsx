import { defineResource, tr } from '../../index'
import type { MenuLocation } from './types'

const PinIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M12 21s-7-6.1-7-11.5a7 7 0 0 1 14 0C19 14.9 12 21 12 21z" />
    <circle cx="12" cy="9.5" r="2.5" />
  </svg>
)

/**
 * The places of the site that show a menu (MKSine's menu locations). They come from the
 * site's theme and plugins; here each gets its label and the menu it shows.
 */
export const menuLocations = defineResource<MenuLocation>({
  name: 'menu_locations',
  label: tr({ fa: 'موقعیت منو', en: 'Menu location' }),
  pluralLabel: tr({ fa: 'موقعیت‌های منو', en: 'Menu locations' }),
  icon: <PinIcon />,
  group: tr({ fa: 'ظاهر', en: 'Appearance' }),
  navigationSort: 20,
  defaultSort: 'label',
  recordTitle: 'label',

  table: (t) => [
    t.badge('location').label(tr({ fa: 'کلید', en: 'Key' })).searchable().sortable(),
    t.text('label').label(tr({ fa: 'عنوان', en: 'Label' })).searchable().sortable().weight('medium'),
    t.badge('menu.name').label(tr({ fa: 'منوی اختصاص‌یافته', en: 'Assigned menu' })).placeholder(tr({ fa: 'اختصاص داده نشده', en: 'Not assigned' })),
    t.date('updated_at').label(tr({ fa: 'به‌روزرسانی', en: 'Updated' })).since().sortable().toggleable(true),
  ],

  form: (f) => [
    f.section(tr({ fa: 'موقعیت منو', en: 'Menu location' })).schema([
      f.textInput('location').label(tr({ fa: 'کلید', en: 'Key' })).disabled().dir('ltr')
        .helperText(tr({ fa: 'کلید را قالب یا افزونه تعیین می‌کند و تغییر نمی‌کند.', en: 'Set by the theme or a plugin; it does not change.' })),
      f.textInput('label').label(tr({ fa: 'عنوان', en: 'Label' })).required().maxLength(255),
      f.select('menu_id').label(tr({ fa: 'منوی اختصاص‌یافته', en: 'Assigned menu' })).optionsFrom('menus').searchable()
        .helperText(tr({ fa: 'منویی که در این جای سایت دیده می‌شود. یک منو می‌تواند در چند موقعیت باشد.', en: 'The menu shown here. One menu can be at several locations.' })),
    ]),
  ],
})
