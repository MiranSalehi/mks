import { MenuIcon, defineResource, tr } from '../../index'
import type { Menu, MenuItem } from './types'

/** Every entry of a menu tree, counted. */
function countEntries(items: MenuItem[]): number {
  return items.reduce((n, item) => n + 1 + countEntries((item.children ?? []) as MenuItem[]), 0)
}

export const menus = defineResource<Menu>({
  name: 'menus',
  label: tr({ fa: 'منو', en: 'Menu' }),
  pluralLabel: tr({ fa: 'منوها', en: 'Menus' }),
  icon: <MenuIcon />,
  group: tr({ fa: 'ظاهر', en: 'Appearance' }),
  navigationSort: 10,
  defaultSort: 'name',
  recordTitle: 'name',

  table: (t) => [
    t.text('name').label(tr({ fa: 'نام', en: 'Name' })).searchable().sortable().weight('medium'),
    t.text('items')
      .label(tr({ fa: 'آیتم‌ها', en: 'Items' }))
      .numeric()
      .format((value) => String(countEntries(Array.isArray(value) ? (value as MenuItem[]) : []))),
    t.date('updated_at').label(tr({ fa: 'آخرین تغییر', en: 'Last changed' })).since().sortable(),
  ],

  actions: (a) => [
    a.link('builder').label(tr({ fa: 'ساخت منو', en: 'Menu builder' })).color('primary').url((m) => `/menu_builder#${m.id}`),
    a.delete(),
  ],

  form: (f) => [
    f.section(tr({ fa: 'منو', en: 'Menu' })).schema([
      f.textInput('name').label(tr({ fa: 'نام منو', en: 'Menu name' })).required().maxLength(100)
        .helperText(tr({ fa: 'آیتم‌ها در «ساخت منو» چیده می‌شوند و جای نمایش منو در «موقعیت‌های منو».', en: 'Items are arranged in the menu builder; where the menu shows is chosen under "Menu locations".' })),
    ]),
  ],
})
