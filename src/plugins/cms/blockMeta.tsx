import { tr } from '../../index'

// What the page builder's "Add component" dialog shows for each block (MKSine's
// component registry): its icon and a one-line description, and the category tabs.

const L = (fa: string, en: string) => tr({ fa, en })

/** Outline icons (24px, stroke), close to the Heroicons MKSine uses. */
export const blockIcons: Record<string, string> = {
  h1: 'M4 6v12M12 6v12M4 12h8M17 10l2-1v9',
  doc: 'M7 3h7l5 5v13H7zM14 3v5h5M10 13h6M10 17h6',
  list: 'M9 6h11M9 12h11M9 18h11M5 6h.01M5 12h.01M5 18h.01',
  chat: 'M4 5h16v11H9l-5 4zM8 9h8M8 12h5',
  chats: 'M3 5h12v8H7l-4 3zM9 17h8l4 3v-9h-3',
  photo: 'M4 5h16v14H4zM4 15l4-4 5 5 3-3 4 4M15 9h.01',
  video: 'M4 7h11v10H4zM15 10l5-3v10l-5-3',
  music: 'M9 18V6l10-2v12M9 18a3 3 0 1 1-3-3 3 3 0 0 1 3 3zM19 16a3 3 0 1 1-3-3 3 3 0 0 1 3 3z',
  updown: 'M8 7l4-4 4 4M12 3v18M8 17l4 4 4-4',
  minus: 'M5 12h14',
  columns: 'M4 5h16v14H4zM9.5 5v14M14.5 5v14',
  grid: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  inset: 'M4 4l5 5M20 4l-5 5M4 20l5-5M20 20l-5-5M9 9h6v6H9z',
  stack2: 'M8 8h12v12H8zM4 16V4h12',
  rects: 'M4 4h16v5H4zM4 11h16v4H4zM4 17h16v3H4z',
  window: 'M3 5h18v14H3zM3 9h18M6 7h.01M8.5 7h.01',
  cursor: 'M5 3l14 7-6 2-2 6zM12 12l6 6',
  megaphone: 'M3 10v4h4l7 4V6L7 10zM17 9a3 3 0 0 1 0 6',
  queue: 'M4 6h16M4 10h16M4 14h16M4 18h10',
  chart: 'M4 4h16v16H4zM8 16v-4M12 16V8M16 16v-6',
  globe: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18',
  tag: 'M4 4h7l9 9-7 7-9-9zM8 8h.01',
  sparkles: 'M12 3l2 5 5 2-5 2-2 5-2-5-5-2 5-2zM19 15l1 2 2 1-2 1-1 2-1-2-2-1 2-1z',
  quote: 'M7 7h4v4H8v3M14 7h4v4h-3v3',
  callout: 'M12 8v5M12 16h.01M4 4h16v14H4z',
  puzzle: 'M10 4a2 2 0 1 1 4 0v1h4a1 1 0 0 1 1 1v4h-1a2 2 0 1 0 0 4h1v4a1 1 0 0 1-1 1h-4v-1a2 2 0 1 0-4 0v1H6a1 1 0 0 1-1-1v-4h1a2 2 0 1 0 0-4H5V6a1 1 0 0 1 1-1h4z',
}

export const blockMeta: Record<string, { icon: string; description: string }> = {
  heading: { icon: 'h1', description: L('یک عنوان یا سرتیتر به صفحه اضافه کنید.', 'Add a heading or title to the page.') },
  text: { icon: 'doc', description: L('متن غنی با قالب‌بندی اضافه کنید.', 'Add rich, formatted text.') },
  quote: { icon: 'quote', description: L('یک نقل‌قول با نام گوینده.', 'A quotation with its author.') },
  callout: { icon: 'callout', description: L('کادری برای نکته، هشدار یا توضیح مهم.', 'A box for a tip, warning or note.') },
  features: { icon: 'list', description: L('شبکه‌ای از ویژگی‌ها با آیکون، عنوان و توضیح نمایش دهید.', 'Show a grid of features with an icon, title and text.') },
  testimonial: { icon: 'chat', description: L('نظرات و بازخورد مشتریان را نمایش دهید.', "Show customers' reviews.") },
  image: { icon: 'photo', description: L('یک تصویر از کتابخانهٔ رسانه اضافه کنید.', 'Add an image from the media library.') },
  video: { icon: 'video', description: L('یک ویدیو از کتابخانهٔ رسانه اضافه کنید.', 'Add a video from the media library.') },
  audio: { icon: 'music', description: L('یک فایل صوتی از کتابخانهٔ رسانه اضافه کنید.', 'Add an audio file from the media library.') },
  slider: { icon: 'photo', description: L('کاروسل تصویر یا محتوا با ناوبری.', 'An image or content carousel.') },
  spacer: { icon: 'updown', description: L('فاصلهٔ عمودی بین المان‌ها اضافه کنید.', 'Add vertical space.') },
  divider: { icon: 'minus', description: L('یک خط جداکنندهٔ افقی اضافه کنید.', 'Add a horizontal line.') },
  columns: { icon: 'columns', description: L('چیدمان چندستونه بسازید. کامپوننت‌ها را در هر ستون بکشید.', 'Build a multi-column layout; drag components into each column.') },
  grid_layout: { icon: 'grid', description: L('شبکهٔ ۱۲ واحدی: عرض هر ستون را برای موبایل، تبلت و دسکتاپ جدا تعیین کنید.', 'A 12-unit grid: set each column’s width per screen size.') },
  container_inset: { icon: 'inset', description: L('حداکثر عرض محتوا و پس‌زمینهٔ اختیاری؛ سایر کامپوننت‌ها را داخل آن قرار دهید.', 'A maximum content width and optional background; put components inside.') },
  card: { icon: 'stack2', description: L('هدر، بدنه و فوتر. کامپوننت‌ها را داخل هر بخش بگذارید.', 'Header, body and footer; put components in each part.') },
  section_ref: { icon: 'rects', description: L('یک سکشن ذخیره‌شده را دوباره استفاده می‌کند. ویرایش سکشن هر صفحه‌ای را که به آن اشاره دارد عوض می‌کند.', 'Reuses a saved section; editing it changes every page that uses it.') },
  hero: { icon: 'window', description: L('بخش هیروی تمام‌عرض با عنوان، زیرعنوان و فراخوان اقدام.', 'A full-width hero with title, subtitle and call to action.') },
  tabs: { icon: 'rects', description: L('بخش‌های تب‌دار برای اطلاعات منظم.', 'Tabbed sections for organized content.') },
  button: { icon: 'cursor', description: L('یک دکمهٔ فراخوان اقدام اضافه کنید.', 'Add a call-to-action button.') },
  cta: { icon: 'megaphone', description: L('بخشی برای تشویق کاربر با عنوان، متن و دکمه‌ها.', 'A section with a title, text and buttons.') },
  accordion: { icon: 'queue', description: L('بخش‌های سوالات متداول تاشو با سوال و جواب.', 'Collapsible questions and answers.') },
  mksine_finance_showcase: { icon: 'chart', description: L('سکشن گرادیانی با آمار، دکمه‌ها و پیش‌نمایش داشبورد.', 'A gradient section with stats, buttons and a dashboard preview.') },
  mksine_hero_domain: { icon: 'globe', description: L('هیروی دو ستونه با تیتر، زیرتیتر، دکمه و تصویر.', 'A two-column hero with heading, button and image.') },
  mksine_services_trio: { icon: 'grid', description: L('سه کارت با تصویر، عنوان، متن و دکمه.', 'Three cards with image, title, text and button.') },
  mksine_featured_domains: { icon: 'tag', description: L('کارت‌های TLD با لوگو، قیمت و تخفیف.', 'TLD cards with price and discount.') },
  mksine_clinic_features_grid: { icon: 'sparkles', description: L('کارت‌های واکنش‌گرا با آیکن گرادیانی.', 'Responsive cards with gradient icons.') },
  mksine_testimonials_grid: { icon: 'chats', description: L('نقل‌قول مشتریان در چند ستون.', 'Customer quotes in columns.') },
  mksine_post_comments_feed: { icon: 'chat', description: L('نظرات تأییدشده از یک پست منتشرشده یا تازه‌ترین‌ها در همهٔ پست‌های منتشرشده.', 'Approved comments of a post, or the latest across posts.') },
}

/** The add dialog's tabs, in MKSine's order. */
export const blockCategories: { key: string; label: string; icon: string }[] = [
  { key: 'content', label: L('محتوا', 'Content'), icon: 'doc' },
  { key: 'media', label: L('رسانه', 'Media'), icon: 'photo' },
  { key: 'layout', label: L('چیدمان', 'Layout'), icon: 'columns' },
  { key: 'interactive', label: L('تعاملی', 'Interactive'), icon: 'cursor' },
  { key: 'sections', label: L('بخش‌های تم', 'Theme sections'), icon: 'rects' },
]

export function BlockIcon({ name, className = 'size-5' }: { name: string; className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
      <path d={blockIcons[name] ?? blockIcons.puzzle} />
    </svg>
  )
}
