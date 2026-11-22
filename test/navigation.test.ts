import { describe, expect, it } from 'vitest'
import { groupNavigation, type NavItem } from '../src/components/Layout'

const item = (key: string, label: string, group?: string): NavItem => ({ key, to: '/', label, icon: null, active: false, group })
const items = [
  item('dashboard', 'Dashboard'),
  item('r:posts', 'Posts', 'Content'),
  item('r:pages', 'Pages', 'Content'),
  item('r:users', 'Users', 'System'),
  item('p:admin_menu', 'Menu', 'System'),
  item('r:tags', 'Tags'),
]
const shape = (groups: ReturnType<typeof groupNavigation>) => groups.map((g) => [g.heading, g.items.map((i) => i.label)])

describe('sidebar groups', () => {
  it('groups by first appearance with ungrouped items first', () => {
    expect(shape(groupNavigation(items, 'Menu'))).toEqual([
      ['Menu', ['Dashboard', 'Tags']],
      ['Content', ['Posts', 'Pages']],
      ['System', ['Users', 'Menu']],
    ])
  })

  it('applies overrides: hide, rename, move, sort, group label and order', () => {
    const groups = groupNavigation(items, 'Menu', {
      items: {
        'r:tags': { group: 'Content', label: 'Labels', sort: 1 },
        'r:users': { hidden: true },
        'r:pages': { sort: 0 },
      },
      groups: { Content: { label: 'Writing', sort: -5 } },
    })
    expect(shape(groups)).toEqual([
      ['Writing', ['Pages', 'Labels', 'Posts']],
      ['Menu', ['Dashboard']],
      ['System', ['Menu']],
    ])
  })

  it('honours placement: hidden removes an item, group moves it', () => {
    const groups = groupNavigation(items, 'Menu', {
      items: { 'r:users': { placement: 'hidden' }, 'r:tags': { placement: 'group', group: 'Content' } },
    })
    expect(shape(groups)).toEqual([
      ['Menu', ['Dashboard']],
      ['Content', ['Posts', 'Pages', 'Tags']],
      ['System', ['Menu']],
    ])
  })

  it('never hides a protected cluster, whatever its placement says', () => {
    const list: NavItem[] = [{ ...item('c:settings', 'Settings'), protected: true }]
    expect(shape(groupNavigation(list, 'Menu', { items: { 'c:settings': { placement: 'hidden' } } }))).toEqual([['Menu', ['Settings']]])
  })

  it('never hides the menu editor or its group', () => {
    const groups = groupNavigation(items, 'Menu', { items: { 'p:admin_menu': { hidden: true } }, groups: { System: { hidden: true } } })
    expect(shape(groups).find(([h]) => h === 'System')).toEqual(['System', ['Users', 'Menu']])
    const hidden = groupNavigation(items, 'Menu', { groups: { Content: { hidden: true } } })
    expect(shape(hidden).some(([h]) => h === 'Content')).toBe(false)
  })
})

describe('sidebar order and clusters', () => {
  it('orders groups as the panel lists them and items by their sort', () => {
    const list: NavItem[] = [
      { ...item('r:tags', 'Tags', 'Content'), sort: 4 },
      { ...item('r:posts', 'Posts', 'Content'), sort: 1 },
      item('r:users', 'Users', 'Access'),
      { ...item('r:media', 'Media', 'Media'), sort: 10 },
      { ...item('l:upload', 'Upload', 'Media'), sort: 11 },
      { ...item('c:settings', 'Settings'), sort: 80 },
      { ...item('dashboard', 'Dashboard'), sort: -2 },
      item('r:extra', 'Extra', 'Other'),
    ]
    const order = [{ label: 'Media' }, { label: 'Content' }, { label: 'Access' }]
    expect(shape(groupNavigation(list, 'Menu', undefined, order))).toEqual([
      ['Menu', ['Dashboard', 'Settings']],
      ['Media', ['Media', 'Upload']],
      ['Content', ['Posts', 'Tags']],
      ['Access', ['Users']],
      ['Other', ['Extra']],
    ])
  })

  it('never hides a protected entry (the cluster holding the menu editor)', () => {
    const list: NavItem[] = [{ ...item('c:settings', 'Settings'), protected: true }, item('r:posts', 'Posts')]
    expect(shape(groupNavigation(list, 'Menu', { items: { 'c:settings': { hidden: true }, 'r:posts': { hidden: true } } }))).toEqual([['Menu', ['Settings']]])
  })
})

describe('navigationItems', () => {
  it('puts cluster pages under one entry and hides items with navigation: false', async () => {
    const { navigationItems } = await import('../src/components/Layout')
    const page = (name: string, extra: object = {}) => ({ name, label: name, component: () => null, ...extra })
    const items = navigationItems(
      {
        meta: { resources: {}, page_badges: {} } as never,
        resources: [{ name: 'trash', pluralLabel: 'Trash', navigation: false } as never, { name: 'media', pluralLabel: 'Media' } as never],
        pages: [page('perf', { cluster: 'settings', navigationSort: 25 }), page('general', { cluster: 'settings', navigationSort: 10 }), page('admin_menu', { cluster: 'settings', navigationSort: 190 }), page('hidden', { navigation: false })],
        navigation: {
          groups: [],
          clusters: [{ name: 'settings', label: 'Settings', sort: 80 }],
          links: [{ key: 'upload', label: 'Upload', to: '/media/create', visible: () => true }, { key: 'never', label: 'Never', to: '/x', visible: () => false }],
          userMenu: [],
        },
      },
      'media',
      '/media/create',
    )
    expect(items.map((i) => i.key)).toEqual(['r:media', 'c:settings', 'l:upload'])
    const settings = items.find((i) => i.key === 'c:settings')!
    expect(settings.params).toEqual({ resource: 'general' })
    expect(settings.protected).toBe(true)
    // On the upload page the link is current, not the media list.
    expect(items.find((i) => i.key === 'l:upload')!.active).toBe(true)
    expect(items.find((i) => i.key === 'r:media')!.active).toBe(false)
  })
})
