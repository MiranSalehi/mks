import { useEffect, useRef, useState } from 'react'
import { focusBuilderBlock, tr, usePanel, type Block, type FieldContext, type FormBuilder, type SchemaNode } from '../../index'
import { mediaPicker } from './media'

// The page builder's blocks, generated from one declarative list that mirrors the Go
// specs (plugins/cms/builder.go): the server checks and cleans every field again.

type Kind = 'text' | 'textarea' | 'html' | 'url' | 'image' | 'file' | 'enum' | 'bool' | 'int' | 'color' | 'list' | 'section' | 'post'

export interface FieldDef {
  name: string
  label: string
  kind: Kind
  options?: Record<string, string>
  default?: unknown
  required?: boolean
  max?: number
  help?: string
  item?: FieldDef[]
  /** List items are titled "{itemTitle} n" instead of by their first field. */
  itemTitle?: string
  full?: boolean
}

export interface BlockDef {
  type: string
  label: string
  category: 'content' | 'media' | 'layout' | 'interactive' | 'sections'
  fields: FieldDef[]
  slots?: number
  minSlots?: number
  slotLabels?: string[]
}

const L = (fa: string, en: string) => tr({ fa, en })

const opts = {
  align: { right: L('راست', 'Right'), center: L('وسط', 'Center'), left: L('چپ', 'Left') },
  sizes: { none: L('هیچ', 'None'), sm: L('کم', 'Small'), md: L('متوسط', 'Medium'), lg: L('زیاد', 'Large') },
  btn: { primary: L('اصلی', 'Primary'), secondary: L('دوم', 'Secondary'), outline: L('خطی', 'Outline'), ghost: L('بی‌زمینه', 'Ghost') },
  gap: { none: L('بدون فاصله', 'None'), sm: L('کم', 'Small'), md: L('متوسط', 'Medium'), lg: L('زیاد', 'Large') },
  valign: { start: L('بالا', 'Top'), center: L('وسط', 'Middle'), end: L('پایین', 'Bottom'), stretch: L('هم‌قد', 'Stretch') },
  dir: { auto: L('خودکار', 'Auto'), rtl: L('راست به چپ', 'Right to left'), ltr: L('چپ به راست', 'Left to right') },
  icons: { users: L('افراد', 'People'), calendar: L('تقویم', 'Calendar'), 'chart-column': L('نمودار', 'Chart'), 'clipboard-list': L('فهرست', 'List'), 'credit-card': L('کارت بانکی', 'Card'), settings: L('تنظیمات', 'Settings'), heart: L('قلب', 'Heart'), shield: L('سپر', 'Shield'), star: L('ستاره', 'Star'), bell: L('زنگ', 'Bell'), globe: L('کره', 'Globe'), phone: L('تلفن', 'Phone') },
  gradients: {
    'from-blue-500 to-blue-600': L('آبی', 'Blue'), 'from-emerald-500 to-emerald-600': L('سبز زمردی', 'Emerald'), 'from-violet-500 to-violet-600': L('بنفش', 'Violet'),
    'from-yellow-500 to-yellow-600': L('زرد', 'Yellow'), 'from-purple-500 to-purple-600': L('ارغوانی', 'Purple'), 'from-pink-500 to-pink-600': L('صورتی', 'Pink'),
    'from-violet-500 to-purple-600': L('بنفش تا ارغوانی', 'Violet to purple'), 'from-indigo-500 to-blue-600': L('نیلی تا آبی', 'Indigo to blue'), 'from-fuchsia-500 to-pink-600': L('سرخابی تا صورتی', 'Fuchsia to pink'),
    'from-amber-500 to-orange-600': L('کهربایی تا نارنجی', 'Amber to orange'), 'from-emerald-500 to-teal-600': L('زمردی تا فیروزه‌ای', 'Emerald to teal'), 'from-sky-500 to-indigo-600': L('آسمانی تا نیلی', 'Sky to indigo'),
  },
  stack: { mobile: L('روی موبایل زیر هم', 'Stack on mobile'), tablet: L('روی تبلت زیر هم', 'Stack on tablet'), always: L('همیشه زیر هم', 'Always stack'), never: L('هرگز', 'Never') },
}

const secondaryButton: FieldDef[] = [
  { name: 'show_secondary_button', label: L('دکمهٔ دوم', 'Second button'), kind: 'bool' },
  { name: 'secondary_button_text', label: L('متن دکمهٔ دوم', 'Second button text'), kind: 'text', max: 100 },
  { name: 'secondary_button_url', label: L('نشانی دکمهٔ دوم', 'Second button URL'), kind: 'url' },
]

export const blockDefs: BlockDef[] = [
  { type: 'heading', label: L('عنوان', 'Heading'), category: 'content', fields: [
    { name: 'text', label: L('متن', 'Text'), kind: 'text', required: true, max: 300, full: true },
    { name: 'level', label: L('سطح', 'Level'), kind: 'enum', options: { h1: 'H1', h2: 'H2', h3: 'H3', h4: 'H4', h5: 'H5', h6: 'H6' }, default: 'h2' },
    { name: 'alignment', label: L('چینش', 'Alignment'), kind: 'enum', options: opts.align, default: 'right' },
  ] },
  { type: 'text', label: L('متن', 'Text'), category: 'content', fields: [
    { name: 'content', label: L('متن', 'Text'), kind: 'html', required: true, full: true },
  ] },
  { type: 'quote', label: L('نقل‌قول', 'Quote'), category: 'content', fields: [
    { name: 'text', label: L('متن نقل‌قول', 'Quote'), kind: 'textarea', required: true, full: true },
    { name: 'author', label: L('گوینده', 'Author'), kind: 'text' },
  ] },
  { type: 'callout', label: L('کادر توجه', 'Callout'), category: 'content', fields: [
    { name: 'tone', label: L('نوع پیام', 'Tone'), kind: 'enum', options: { info: L('اطلاع‌رسانی', 'Info'), warning: L('هشدار', 'Warning') }, default: 'info' },
    { name: 'text', label: L('متن پیام', 'Text'), kind: 'textarea', required: true, full: true },
  ] },
  { type: 'features', label: L('لیست ویژگی‌ها', 'Features list'), category: 'content', fields: [
    { name: 'heading', label: L('تیتر', 'Heading'), kind: 'text' },
    { name: 'subheading', label: L('زیرتیتر', 'Subheading'), kind: 'textarea' },
    { name: 'columns', label: L('تعداد ستون', 'Columns'), kind: 'int', default: 3 },
    { name: 'style', label: L('سبک', 'Style'), kind: 'enum', options: { simple: L('ساده', 'Simple'), bordered: L('قاب‌دار', 'Bordered'), shadowed: L('سایه‌دار', 'Shadowed'), filled: L('پررنگ', 'Filled') }, default: 'simple' },
    { name: 'icon_style', label: L('قاب آیکن', 'Icon style'), kind: 'enum', options: { circle: L('دایره', 'Circle'), square: L('مربع', 'Square'), none: L('بدون قاب', 'None') }, default: 'circle' },
    { name: 'features', label: L('ویژگی‌ها', 'Features'), kind: 'list', full: true, item: [
      { name: 'icon', label: L('آیکن (ایموجی)', 'Icon (emoji)'), kind: 'text', max: 8 },
      { name: 'title', label: L('عنوان', 'Title'), kind: 'text', required: true },
      { name: 'description', label: L('توضیح', 'Description'), kind: 'textarea', full: true },
      { name: 'link', label: L('پیوند', 'Link'), kind: 'url' },
    ] },
  ] },
  { type: 'testimonial', label: L('نظرات مشتریان', 'Testimonials'), category: 'content', fields: [
    { name: 'heading', label: L('تیتر', 'Heading'), kind: 'text' },
    { name: 'columns', label: L('تعداد ستون', 'Columns'), kind: 'int', default: 3 },
    { name: 'style', label: L('سبک', 'Style'), kind: 'enum', options: { simple: L('ساده', 'Simple'), bordered: L('قاب‌دار', 'Bordered'), shadowed: L('سایه‌دار', 'Shadowed'), quote: L('نقل‌قول', 'Quote') }, default: 'bordered' },
    { name: 'testimonials', label: L('نظرها', 'Testimonials'), kind: 'list', full: true, item: [
      { name: 'content', label: L('متن', 'Text'), kind: 'textarea', required: true, full: true },
      { name: 'author_name', label: L('نام', 'Name'), kind: 'text' },
      { name: 'author_title', label: L('سمت', 'Title'), kind: 'text' },
      { name: 'author_image', label: L('تصویر', 'Photo'), kind: 'image' },
      { name: 'rating', label: L('امتیاز (۰ تا ۵)', 'Rating (0–5)'), kind: 'int', default: 0 },
    ] },
  ] },
  { type: 'image', label: L('تصویر', 'Image'), category: 'media', fields: [
    { name: 'image', label: L('تصویر', 'Image'), kind: 'image', required: true },
    { name: 'alt', label: L('متن جایگزین', 'Alt text'), kind: 'text', help: L('خالی: متن جایگزین کتابخانه.', 'Empty: the library alt text.') },
    { name: 'caption', label: L('زیرنویس', 'Caption'), kind: 'text' },
    { name: 'size', label: L('اندازه', 'Size'), kind: 'enum', options: { small: L('کوچک', 'Small'), medium: L('متوسط', 'Medium'), large: L('بزرگ', 'Large'), full: L('تمام‌عرض', 'Full') }, default: 'large' },
    { name: 'alignment', label: L('چینش', 'Alignment'), kind: 'enum', options: opts.align, default: 'center' },
    { name: 'rounded', label: L('گوشه‌گرد', 'Rounded'), kind: 'bool' },
    { name: 'shadow', label: L('سایه', 'Shadow'), kind: 'bool' },
  ] },
  { type: 'video', label: L('ویدیو', 'Video'), category: 'media', fields: [
    { name: 'video', label: L('فایل ویدیو', 'Video file'), kind: 'file', required: true },
    { name: 'caption', label: L('زیرنویس', 'Caption'), kind: 'text' },
    { name: 'controls', label: L('کنترل‌ها', 'Controls'), kind: 'bool', default: true },
    { name: 'autoplay', label: L('پخش خودکار (بی‌صدا)', 'Autoplay (muted)'), kind: 'bool' },
    { name: 'loop', label: L('تکرار', 'Loop'), kind: 'bool' },
  ] },
  { type: 'audio', label: L('صوت', 'Audio'), category: 'media', fields: [
    { name: 'audio', label: L('فایل صدا', 'Audio file'), kind: 'file', required: true },
    { name: 'caption', label: L('زیرنویس', 'Caption'), kind: 'text' },
    { name: 'controls', label: L('کنترل‌ها', 'Controls'), kind: 'bool', default: true },
    { name: 'autoplay', label: L('پخش خودکار', 'Autoplay'), kind: 'bool' },
    { name: 'loop', label: L('تکرار', 'Loop'), kind: 'bool' },
  ] },
  { type: 'slider', label: L('اسلایدر/کاروسل', 'Slider/carousel'), category: 'media', fields: [
    { name: 'height', label: L('ارتفاع', 'Height'), kind: 'enum', options: { small: L('کم', 'Small'), medium: L('متوسط', 'Medium'), large: L('زیاد', 'Large'), auto: L('خودکار', 'Auto') }, default: 'medium' },
    { name: 'autoplay', label: L('چرخش خودکار', 'Autoplay'), kind: 'bool', default: true },
    { name: 'autoplay_speed', label: L('فاصلهٔ چرخش (میلی‌ثانیه)', 'Interval (ms)'), kind: 'int', default: 5000 },
    { name: 'show_arrows', label: L('پیکان‌ها', 'Arrows'), kind: 'bool', default: true },
    { name: 'show_dots', label: L('نقطه‌ها', 'Dots'), kind: 'bool', default: true },
    { name: 'slides', label: L('اسلایدها', 'Slides'), kind: 'list', full: true, item: [
      { name: 'image', label: L('تصویر', 'Image'), kind: 'image' },
      { name: 'title', label: L('عنوان', 'Title'), kind: 'text' },
      { name: 'subtitle', label: L('زیرعنوان', 'Subtitle'), kind: 'textarea', full: true },
      { name: 'button_text', label: L('متن دکمه', 'Button text'), kind: 'text' },
      { name: 'button_url', label: L('نشانی دکمه', 'Button URL'), kind: 'url' },
    ] },
  ] },
  { type: 'spacer', label: L('فاصله‌انداز', 'Spacer'), category: 'layout', fields: [
    { name: 'size', label: L('اندازه', 'Size'), kind: 'enum', options: { xs: 'XS', sm: 'S', md: 'M', lg: 'L', xl: 'XL', '2xl': '2XL' }, default: 'md' },
  ] },
  { type: 'divider', label: L('جداکننده', 'Divider'), category: 'layout', fields: [
    { name: 'style', label: L('نوع خط', 'Style'), kind: 'enum', options: { solid: L('پیوسته', 'Solid'), dashed: L('خط‌چین', 'Dashed'), dotted: L('نقطه‌چین', 'Dotted') }, default: 'solid' },
    { name: 'line_width', label: L('طول', 'Length'), kind: 'enum', options: { 25: '۲۵٪', 50: '۵۰٪', 75: '۷۵٪', 100: '۱۰۰٪' }, default: '100' },
    { name: 'alignment', label: L('چینش', 'Alignment'), kind: 'enum', options: opts.align, default: 'center' },
  ] },
  { type: 'columns', label: L('ستون‌ها', 'Columns'), category: 'layout', slots: 12, minSlots: 2, fields: [
    { name: 'layout', label: L('نسبت ستون‌ها', 'Layout'), kind: 'enum', options: { equal: L('برابر', 'Equal'), '1-2': '۱:۲', '2-1': '۲:۱', '1-3': '۱:۳', '3-1': '۳:۱', '1-2-1': '۱:۲:۱' }, default: 'equal' },
    { name: 'gap', label: L('فاصلهٔ ستون‌ها', 'Gap'), kind: 'enum', options: opts.gap, default: 'md' },
    { name: 'vertical_alignment', label: L('چینش عمودی', 'Vertical alignment'), kind: 'enum', options: opts.valign, default: 'start' },
    { name: 'stack_on_mobile', label: L('در صفحهٔ کوچک', 'On small screens'), kind: 'enum', options: opts.stack, default: 'mobile' },
  ] },
  { type: 'grid_layout', label: L('گرید CSS (سطر ۱۲ واحدی)', 'CSS grid (12-unit row)'), category: 'layout', slots: 12, minSlots: 1, fields: [
    { name: 'gap', label: L('فاصله', 'Gap'), kind: 'enum', options: opts.gap, default: 'md' },
    { name: 'vertical_alignment', label: L('چینش عمودی', 'Vertical alignment'), kind: 'enum', options: opts.valign, default: 'start' },
    { name: 'stack_on_mobile', label: L('زیر هم شدن ستون‌ها', 'Stack columns'), kind: 'enum', options: opts.stack, default: 'mobile' },
    { name: 'column_spans', label: L('عرض ستون‌ها (از ۱۲)', 'Column widths (of 12)'), kind: 'list', max: 12, itemTitle: L('ستون', 'Column'),
      help: L('هر ردیف برای یک ستون، به همان ترتیب: عرضی از ۱ تا ۱۲. اندازهٔ خالی مثل اندازهٔ کوچک‌تر است؛ ستون‌های بیشتر از ردیف‌ها، عرض آخرین ردیف را می‌گیرند. بدون ردیف: ستون‌ها خودکار هم‌عرض می‌شوند.', 'One row per column, in order: a width of 1 to 12. An empty size keeps the smaller one; extra columns take the last row. No rows: columns share the width evenly.'),
      item: [
        { name: 'span', label: L('همه‌جا', 'Everywhere'), kind: 'int', default: 6 },
        { name: 'span_sm', label: L('از ۶۴۰px', 'From 640px'), kind: 'int' },
        { name: 'span_md', label: L('از ۷۶۸px (تبلت)', 'From 768px (tablet)'), kind: 'int' },
        { name: 'span_lg', label: L('از ۱۰۲۴px (دسکتاپ)', 'From 1024px (desktop)'), kind: 'int' },
      ] },
  ] },
  { type: 'container_inset', label: L('کانتینر محتوا', 'Content container'), category: 'layout', slots: 1, minSlots: 1, slotLabels: [L('محتوا', 'Content')], fields: [
    { name: 'max_width', label: L('بیشترین عرض', 'Max width'), kind: 'enum', options: { full: L('تمام', 'Full'), prose: L('متن', 'Prose'), '3xl': '3XL', '5xl': '5XL', '6xl': '6XL', '7xl': '7XL' }, default: '5xl' },
    { name: 'background_full_bleed', label: L('زمینه تا لبهٔ صفحه', 'Full-bleed background'), kind: 'bool' },
    { name: 'inset_background', label: L('رنگ زمینه', 'Background color'), kind: 'color' },
  ] },
  { type: 'card', label: L('کارت', 'Card'), category: 'layout', slots: 3, minSlots: 1, slotLabels: [L('سرصفحه', 'Header'), L('بدنه', 'Body'), L('پاصفحه', 'Footer')], fields: [
    { name: 'show_header', label: L('سرصفحه', 'Header'), kind: 'bool', default: true },
    { name: 'show_footer', label: L('پاصفحه', 'Footer'), kind: 'bool' },
    { name: 'shadow', label: L('سایه', 'Shadow'), kind: 'enum', options: { none: L('هیچ', 'None'), sm: L('کم', 'Small'), md: L('زیاد', 'Large') }, default: 'sm' },
    { name: 'radius', label: L('گردی گوشه', 'Radius'), kind: 'enum', options: opts.sizes, default: 'md' },
  ] },
  { type: 'section_ref', label: L('سکشن ذخیره‌شده', 'Saved section'), category: 'layout', fields: [
    { name: 'section_id', label: L('بخش', 'Section'), kind: 'section', required: true, full: true },
  ] },
  { type: 'hero', label: L('بخش هیرو', 'Hero section'), category: 'layout', fields: [
    { name: 'title', label: L('عنوان', 'Title'), kind: 'text', required: true, full: true },
    { name: 'subtitle', label: L('زیرعنوان', 'Subtitle'), kind: 'textarea', full: true },
    { name: 'background_image', label: L('تصویر زمینه', 'Background image'), kind: 'image' },
    { name: 'overlay', label: L('پوشش', 'Overlay'), kind: 'enum', options: { none: L('هیچ', 'None'), light: L('روشن', 'Light'), dark: L('تیره', 'Dark'), gradient: L('گرادیان', 'Gradient') }, default: 'dark' },
    { name: 'height', label: L('ارتفاع', 'Height'), kind: 'enum', options: { small: L('کم', 'Small'), medium: L('متوسط', 'Medium'), large: L('زیاد', 'Large'), full: L('تمام صفحه', 'Full screen') }, default: 'medium' },
    { name: 'text_alignment', label: L('چینش متن', 'Text alignment'), kind: 'enum', options: opts.align, default: 'center' },
    { name: 'hero_text_color', label: L('رنگ متن', 'Text color'), kind: 'enum', options: { white: L('سفید', 'White'), dark: L('تیره', 'Dark') }, default: 'white' },
    { name: 'button_text', label: L('متن دکمه', 'Button text'), kind: 'text' },
    { name: 'button_url', label: L('نشانی دکمه', 'Button URL'), kind: 'url' },
    { name: 'button_style', label: L('سبک دکمه', 'Button style'), kind: 'enum', options: { primary: L('اصلی', 'Primary'), secondary: L('دوم', 'Secondary'), outline: L('خطی', 'Outline') }, default: 'primary' },
    ...secondaryButton,
  ] },
  { type: 'tabs', label: L('تب‌ها', 'Tabs'), category: 'layout', fields: [
    { name: 'style', label: L('سبک', 'Style'), kind: 'enum', options: { underline: L('خط زیر', 'Underline'), pills: L('قرصی', 'Pills'), boxed: L('جعبه‌ای', 'Boxed'), buttons: L('دکمه‌ای', 'Buttons') }, default: 'underline' },
    { name: 'tabs', label: L('زبانه‌ها', 'Tabs'), kind: 'list', full: true, item: [
      { name: 'title', label: L('عنوان', 'Title'), kind: 'text', required: true },
      { name: 'content', label: L('محتوا', 'Content'), kind: 'html', full: true },
    ] },
  ] },
  { type: 'button', label: L('دکمه', 'Button'), category: 'interactive', fields: [
    { name: 'text', label: L('متن', 'Text'), kind: 'text', required: true },
    { name: 'url', label: L('نشانی', 'URL'), kind: 'url' },
    { name: 'style', label: L('سبک', 'Style'), kind: 'enum', options: opts.btn, default: 'primary' },
    { name: 'size', label: L('اندازه', 'Size'), kind: 'enum', options: { sm: L('کوچک', 'Small'), md: L('متوسط', 'Medium'), lg: L('بزرگ', 'Large') }, default: 'md' },
    { name: 'alignment', label: L('چینش', 'Alignment'), kind: 'enum', options: opts.align, default: 'right' },
    { name: 'new_tab', label: L('زبانهٔ تازه', 'New tab'), kind: 'bool' },
    { name: 'full_width', label: L('تمام‌عرض', 'Full width'), kind: 'bool' },
  ] },
  { type: 'cta', label: L('فراخوان اقدام', 'Call to action'), category: 'interactive', fields: [
    { name: 'title', label: L('عنوان', 'Title'), kind: 'text', required: true, full: true },
    { name: 'description', label: L('توضیح', 'Description'), kind: 'textarea', full: true },
    { name: 'style', label: L('سبک', 'Style'), kind: 'enum', options: { simple: L('ساده', 'Simple'), boxed: L('جعبه‌ای', 'Boxed'), gradient: L('گرادیان', 'Gradient'), dark: L('تیره', 'Dark') }, default: 'gradient' },
    { name: 'alignment', label: L('چینش', 'Alignment'), kind: 'enum', options: { center: L('وسط', 'Center'), left: L('کنار', 'Side'), between: L('دو سو', 'Spread') }, default: 'center' },
    { name: 'button_text', label: L('متن دکمه', 'Button text'), kind: 'text' },
    { name: 'button_url', label: L('نشانی دکمه', 'Button URL'), kind: 'url' },
    ...secondaryButton,
  ] },
  { type: 'accordion', label: L('سوالات متداول', 'FAQ'), category: 'interactive', fields: [
    { name: 'heading', label: L('تیتر', 'Heading'), kind: 'text' },
    { name: 'style', label: L('سبک', 'Style'), kind: 'enum', options: { simple: L('ساده', 'Simple'), bordered: L('قاب‌دار', 'Bordered'), separated: L('جدا', 'Separated') }, default: 'bordered' },
    { name: 'allow_multiple', label: L('چند مورد باز با هم', 'Several open'), kind: 'bool' },
    { name: 'first_open', label: L('اولی باز', 'First open'), kind: 'bool', default: true },
    { name: 'items', label: L('پرسش‌ها', 'Items'), kind: 'list', full: true, item: [
      { name: 'question', label: L('پرسش', 'Question'), kind: 'text', required: true, full: true },
      { name: 'answer', label: L('پاسخ', 'Answer'), kind: 'html', full: true },
    ] },
  ] },
  // Sections: MKSine's default-theme blocks (names kept, so MKSine pages move over).
  { type: 'mksine_hero_domain', label: L('هیروی دامنه (MKSine)', 'Domain hero (MKSine)'), category: 'sections', fields: [
    { name: 'heading_line1', label: L('خط اول تیتر', 'Heading, first line'), kind: 'text', required: true, full: true, default: 'دامنهٔ شما.' },
    { name: 'heading_line2_prefix', label: L('آغاز خط دوم', 'Second line, start'), kind: 'text', max: 120, default: 'برند شما.' },
    { name: 'heading_accent', label: L('عبارت رنگی', 'Accent phrase'), kind: 'text', max: 120, default: 'آنلاین در چند دقیقه.' },
    { name: 'heading_after', label: L('پس از عبارت رنگی', 'After the accent'), kind: 'text', max: 120 },
    { name: 'subheading', label: L('زیرتیتر', 'Subheading'), kind: 'textarea', full: true },
    { name: 'cta_label', label: L('متن دکمه', 'Button text'), kind: 'text', max: 120, default: 'شروع کنید' },
    { name: 'cta_url', label: L('نشانی دکمه', 'Button URL'), kind: 'url' },
    { name: 'illustration', label: L('تصویر کنار', 'Illustration'), kind: 'image' },
    { name: 'illustration_alt', label: L('متن جایگزین تصویر', 'Illustration alt'), kind: 'text' },
    { name: 'background_image', label: L('تصویر زمینه', 'Background image'), kind: 'image' },
    { name: 'illustration_position', label: L('جای تصویر', 'Illustration side'), kind: 'enum', options: { right: L('راست', 'Right'), left: L('چپ', 'Left') }, default: 'right',
      help: L('رنگ زمینه را از «رنگ زمینه» در تنظیمات سبک بلوک بدهید.', 'Set the background color in the block style settings.') },
  ] },
  { type: 'mksine_finance_showcase', label: L('بخش معرفی مالی (MKSine)', 'Finance showcase (MKSine)'), category: 'sections', fields: [
    { name: 'heading', label: L('تیتر', 'Heading'), kind: 'text', required: true, full: true, default: 'حساب‌وکتاب کسب‌وکارتان، بی‌دردسر.' },
    { name: 'heading_accent', label: L('بخش رنگی تیتر', 'Accent part of the heading'), kind: 'text', max: 120, default: 'بی‌دردسر.', help: L('عین همان متنی که در تیتر آمده.', 'Exactly as it appears in the heading.') },
    { name: 'paragraph_1', label: L('بند اول', 'First paragraph'), kind: 'textarea', full: true },
    { name: 'paragraph_1_highlight', label: L('عبارت برجستهٔ بند اول', 'Highlight in the first paragraph'), kind: 'text', max: 120 },
    { name: 'paragraph_2', label: L('بند دوم', 'Second paragraph'), kind: 'textarea', full: true },
    { name: 'paragraph_2_highlight', label: L('عبارت برجستهٔ بند دوم', 'Highlight in the second paragraph'), kind: 'text', max: 120 },
    { name: 'cta_primary_label', label: L('دکمهٔ اصلی', 'Main button'), kind: 'text', max: 120 },
    { name: 'cta_primary_url', label: L('نشانی دکمهٔ اصلی', 'Main button URL'), kind: 'url' },
    { name: 'cta_secondary_label', label: L('دکمهٔ دوم', 'Second button'), kind: 'text', max: 120 },
    { name: 'cta_secondary_url', label: L('نشانی دکمهٔ دوم', 'Second button URL'), kind: 'url' },
    { name: 'trust_line', label: L('جملهٔ اعتماد', 'Trust line'), kind: 'textarea', full: true },
    { name: 'social_count', label: L('عدد مشتریان', 'Customer count'), kind: 'text', max: 60 },
    { name: 'social_caption', label: L('توضیح مشتریان', 'Customer caption'), kind: 'text' },
    { name: 'mock_chart_caption', label: L('عنوان نمودار', 'Chart caption'), kind: 'text', max: 120, default: 'درآمد ماهانه' },
    { name: 'avatars', label: L('حرف‌های چهره‌ها', 'Avatar letters'), kind: 'list', max: 8, item: [
      { name: 'letter', label: L('حرف', 'Letter'), kind: 'text', max: 2, required: true },
    ] },
  ] },
  { type: 'mksine_services_trio', label: L('سه کارت خدمات (MKSine)', 'Three service cards (MKSine)'), category: 'sections', fields: [
    { name: 'section_title', label: L('تیتر بخش', 'Section title'), kind: 'text', full: true },
    { name: 'cards', label: L('کارت‌ها', 'Cards'), kind: 'list', max: 6, item: [
      { name: 'title', label: L('عنوان', 'Title'), kind: 'text', max: 200, required: true },
      { name: 'body', label: L('متن', 'Text'), kind: 'textarea' },
      { name: 'image', label: L('نماد', 'Icon image'), kind: 'image' },
      { name: 'image_alt', label: L('متن جایگزین', 'Alt text'), kind: 'text' },
      { name: 'cta_label', label: L('متن پیوند', 'Link text'), kind: 'text', max: 120 },
      { name: 'url', label: L('نشانی', 'URL'), kind: 'url' },
    ] },
  ] },
  { type: 'mksine_featured_domains', label: L('گرید دامنهٔ ویژه (MKSine)', 'Featured domains (MKSine)'), category: 'sections', fields: [
    { name: 'title', label: L('تیتر', 'Title'), kind: 'text', default: 'پسوندهای ویژه' },
    { name: 'view_all_label', label: L('متن «همه»', '"View all" text'), kind: 'text', max: 120 },
    { name: 'view_all_url', label: L('نشانی «همه»', '"View all" URL'), kind: 'url' },
    { name: 'items', label: L('پسوندها', 'Extensions'), kind: 'list', item: [
      { name: 'tld', label: L('پسوند', 'Extension'), kind: 'text', max: 40, required: true },
      { name: 'category', label: L('دسته', 'Category'), kind: 'text', max: 120 },
      { name: 'price', label: L('قیمت', 'Price'), kind: 'text', max: 40 },
      { name: 'original_price', label: L('قیمت پیشین', 'Old price'), kind: 'text', max: 40 },
      { name: 'discount_percent', label: L('درصد تخفیف', 'Discount %'), kind: 'int' },
      { name: 'href', label: L('نشانی', 'URL'), kind: 'url' },
      { name: 'logo', label: L('نشان', 'Logo'), kind: 'image' },
    ] },
  ] },
  { type: 'mksine_clinic_features_grid', label: L('گرید امکانات (MKSine)', 'Features grid (MKSine)'), category: 'sections', fields: [
    { name: 'text_direction', label: L('جهت متن', 'Text direction'), kind: 'enum', options: opts.dir, default: 'auto' },
    { name: 'badge', label: L('برچسب بالا', 'Badge'), kind: 'text', max: 120 },
    { name: 'heading_prefix', label: L('آغاز تیتر', 'Heading start'), kind: 'text', max: 200 },
    { name: 'heading_accent', label: L('بخش رنگی تیتر', 'Heading accent'), kind: 'text', max: 200 },
    { name: 'subheading', label: L('زیرتیتر', 'Subheading'), kind: 'textarea', full: true },
    { name: 'cards', label: L('کارت‌ها', 'Cards'), kind: 'list', item: [
      { name: 'title', label: L('عنوان', 'Title'), kind: 'text', max: 200, required: true },
      { name: 'body', label: L('متن', 'Text'), kind: 'textarea' },
      { name: 'icon', label: L('نماد', 'Icon'), kind: 'enum', options: opts.icons, default: 'users' },
      { name: 'gradient', label: L('رنگ', 'Color'), kind: 'enum', options: opts.gradients, default: 'from-blue-500 to-blue-600' },
      { name: 'link_url', label: L('نشانی', 'URL'), kind: 'url' },
      { name: 'link_new_tab', label: L('در زبانهٔ تازه', 'New tab'), kind: 'bool' },
    ] },
  ] },
  { type: 'mksine_testimonials_grid', label: L('گرید نظرات (MKSine)', 'Testimonials grid (MKSine)'), category: 'sections', fields: [
    { name: 'text_direction', label: L('جهت متن', 'Text direction'), kind: 'enum', options: opts.dir, default: 'auto' },
    { name: 'badge', label: L('برچسب بالا', 'Badge'), kind: 'text', max: 120 },
    { name: 'title_prefix', label: L('آغاز تیتر', 'Title start'), kind: 'text', max: 200 },
    { name: 'title_accent', label: L('بخش رنگی تیتر', 'Title accent'), kind: 'text', max: 200 },
    { name: 'subheading', label: L('زیرتیتر', 'Subheading'), kind: 'textarea', full: true },
    { name: 'aside', label: L('کادر کنار', 'Side note'), kind: 'textarea', full: true },
    { name: 'items', label: L('نظرها', 'Testimonials'), kind: 'list', max: 12, item: [
      { name: 'name', label: L('نام', 'Name'), kind: 'text', max: 120, required: true },
      { name: 'city', label: L('شهر', 'City'), kind: 'text', max: 120 },
      { name: 'quote', label: L('متن نظر', 'Quote'), kind: 'textarea' },
      { name: 'gradient', label: L('رنگ', 'Color'), kind: 'enum', options: opts.gradients, default: 'from-violet-500 to-purple-600' },
      { name: 'link_url', label: L('نشانی', 'URL'), kind: 'url' },
      { name: 'link_new_tab', label: L('در زبانهٔ تازه', 'New tab'), kind: 'bool' },
    ] },
  ] },
  { type: 'mksine_post_comments_feed', label: L('نظرات پست (MKSine)', 'Post comments (MKSine)'), category: 'sections', fields: [
    { name: 'post_id', label: L('نوشته', 'Post'), kind: 'post', help: L('خالی: آخرین دیدگاه‌های تأییدشدهٔ همهٔ نوشته‌ها.', 'Empty: the latest approved comments of all posts.') },
    { name: 'max_root_comments', label: L('حداکثر دیدگاه', 'Most comments'), kind: 'int', default: 12 },
    { name: 'quote_max_chars', label: L('حداکثر نویسهٔ هر دیدگاه', 'Most characters each'), kind: 'int', default: 220 },
    { name: 'text_direction', label: L('جهت متن', 'Text direction'), kind: 'enum', options: opts.dir, default: 'auto' },
    { name: 'badge', label: L('برچسب بالا', 'Badge'), kind: 'text', max: 120, default: 'از جامعهٔ ما' },
    { name: 'title_prefix', label: L('آغاز تیتر', 'Title start'), kind: 'text', max: 200, default: 'خوانندگان ' },
    { name: 'title_accent', label: L('بخش رنگی تیتر', 'Title accent'), kind: 'text', max: 200, default: 'چه می‌گویند' },
    { name: 'subheading', label: L('زیرتیتر', 'Subheading'), kind: 'text', max: 500 },
    { name: 'aside', label: L('کادر کنار', 'Side note'), kind: 'textarea', full: true },
  ] },
]

export const presentation: FieldDef[] = [
  { name: 'align', label: L('تراز محتوا', 'Content alignment'), kind: 'enum', options: { start: L('پیش‌فرض', 'Default'), center: L('وسط', 'Center'), end: L('چپ', 'Left') }, default: 'start' },
  { name: 'text_color', label: L('رنگ متن', 'Text color'), kind: 'color' },
  { name: 'background_color', label: L('رنگ پس‌زمینه', 'Background color'), kind: 'color' },
  { name: 'border_width', label: L('ضخامت کادر', 'Border width'), kind: 'enum', options: opts.sizes, default: 'none' },
  { name: 'border_color', label: L('رنگ کادر', 'Border color'), kind: 'color' },
  { name: 'border_radius', label: L('گردی گوشه', 'Corner radius'), kind: 'enum', options: opts.sizes, default: 'none' },
  { name: 'padding', label: L('فاصله داخلی', 'Padding'), kind: 'enum', options: opts.sizes, default: 'none' },
  { name: 'spacing', label: L('فاصله بیرونی', 'Margin'), kind: 'enum', options: opts.sizes, default: 'md' },
  { name: 'background', label: L('پس‌زمینه آماده', 'Preset background'), kind: 'enum', options: { none: L('هیچ', 'None'), muted: L('ملایم', 'Soft'), brand: L('رنگ سایت', 'Brand') }, default: 'none' },
  { name: 'visibility', label: L('نمایش', 'Visibility'), kind: 'enum', options: { always: L('همیشه', 'Always'), hide_mobile: L('مخفی در صفحه کوچک', 'Hidden on small screens'), hide_desktop: L('مخفی در صفحه بزرگ', 'Hidden on large screens') }, default: 'always' },
  { name: 'width', label: L('عرض', 'Width'), kind: 'enum', options: { full: L('تمام‌عرض', 'Full width'), contained: L('محدود', 'Contained') }, default: 'full' },
  { name: 'anchor', label: L('لنگر', 'Anchor'), kind: 'text', full: true, help: L('برای پیوند #… ؛ حروف کوچک انگلیسی.', 'For #links; lowercase letters.') },
]

export function field(g: FormBuilder<any>, f: FieldDef): SchemaNode<any> {
  let node
  switch (f.kind) {
    case 'text':
      node = g.textInput(f.name).maxLength(f.max || 255)
      break
    case 'textarea':
      node = g.textarea(f.name).rows(3)
      break
    case 'html':
      node = g.richEditor(f.name)
      break
    case 'url':
      node = g.textInput(f.name).dir('ltr').placeholder('https://… · /page/… · #anchor')
      break
    case 'image':
      node = mediaPicker(f.name)
      break
    case 'file':
      node = mediaPicker(f.name).option('accept', 'any')
      break
    case 'enum':
      node = g.select(f.name).options(f.options ?? {})
      break
    case 'bool':
      node = g.toggle(f.name)
      break
    case 'int':
      node = g.textInput(f.name).numeric()
      break
    case 'color':
      node = g.textInput(f.name).dir('ltr').placeholder('#rrggbb').maxLength(7)
      break
    case 'section':
      node = g.select(f.name).optionsFrom('cms_sections').searchable()
      break
    case 'post':
      node = g.select(f.name).optionsFrom('cms_posts').searchable()
      break
    case 'list': {
      const first = f.item?.find((i) => i.required) ?? f.item?.[0]
      node = g.repeater(f.name).schema((c) => (f.item ?? []).map((i) => field(c, i))).maxItems(f.max ?? 24)
        .itemLabel((item, i) => (f.itemTitle ? `${f.itemTitle} ${i + 1}` : String(item[first?.name ?? ''] ?? '') || String(i + 1)))
      break
    }
  }
  node.label(f.label)
  if (f.default !== undefined) node.default(f.default)
  if (f.required) node.required()
  if (f.help) node.helperText(f.help)
  if (f.full || f.kind === 'html' || f.kind === 'list') node.columnSpan('full')
  return node
}

// Blocks of uploaded plugins (meta.extra.cms_blocks). The block lists are built when the
// resources are defined, before the server is asked, so they are kept and the plugins'
// blocks are put into them when meta arrives (setPackageBlocks).
let packageDefs: BlockDef[] = []
let packageKey = ''
const liveLists: { blocks: Block[]; g: FormBuilder<any>; depth: number }[] = []

/** Every block type: the built-in ones and the active plugins'. */
export function allBlockDefs(): BlockDef[] {
  return [...blockDefs, ...packageDefs]
}

/** Puts the active plugins' blocks into every builder (called with each meta response). */
export function setPackageBlocks(defs: BlockDef[] | undefined) {
  const list = defs ?? []
  const key = JSON.stringify(list)
  if (key === packageKey) return
  packageKey = key
  packageDefs = list
  for (const live of liveLists) {
    for (let i = live.blocks.length - 1; i >= 0; i--) if (live.blocks[i].name.startsWith('pkg-')) live.blocks.splice(i, 1)
    live.blocks.push(...list.map((b) => makeBlock(live.g, b, live.depth)))
  }
}

/** The blocks of a builder field; containers hold builders of their own (two levels). */
export function builderBlocks(g: FormBuilder<any>, depth = 0): Block[] {
  const blocks = [...blockDefs, ...packageDefs].filter((b) => !b.slots || depth < 2).map((b) => makeBlock(g, b, depth))
  liveLists.push({ blocks, g, depth })
  return blocks
}

function makeBlock(g: FormBuilder<any>, b: BlockDef, depth: number): Block {
  return g.block(b.type).label(b.label).schema((s) => {
    const nodes: SchemaNode<any>[] = b.fields.map((f) => field(s, f))
    if (b.slots) {
      nodes.push(
        s.repeater('children')
          .label(b.slots === 1 ? L('محتوا', 'Content') : L('بخش‌ها', 'Slots'))
          .minItems(b.minSlots ?? 1)
          .maxItems(b.slots)
          .reorderable()
          .itemLabel((_, i) => b.slotLabels?.[i] ?? `${L('ستون', 'Column')} ${i + 1}`)
          .columnSpan('full')
          .schema((c) => [c.builder('blocks').label('').blocks(builderBlocks(c, depth + 1)).columnSpan('full')]),
      )
    }
    nodes.push(s.section(L('سبک', 'Style')).collapsible(true).columns(3).columnSpan('full').schema(presentation.map((f) => field(s, f))))
    return nodes
  })
}

// --- live device preview ----------------------------------------------------------------

const devices = { desktop: { w: '100%', label: L('دسکتاپ', 'Desktop') }, tablet: { w: '768px', label: L('تبلت', 'Tablet') }, mobile: { w: '390px', label: L('موبایل', 'Mobile') } }

/** Renders the unsaved blocks like the site does, in a sandboxed frame. */
export function LivePreview({ get }: { get: FieldContext<any>['get'] }) {
  const { api } = usePanel()
  const payload = get('builder_payload')
  const width = get('builder_content_width')
  const [device, setDevice] = useState<keyof typeof devices>('desktop')
  const [html, setHtml] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const timer = useRef<number | undefined>(undefined)
  const frame = useRef<HTMLIFrameElement>(null)
  const json = JSON.stringify(payload ?? [])

  // Click to edit (MKSine's editable regions): a click on a block in the preview opens
  // that block in the form. The preview runs no scripts; the listener is the editor's.
  function wire() {
    const doc = frame.current?.contentDocument
    if (!doc) return
    const style = doc.createElement('style')
    style.textContent =
      '[data-pb-path]>*{cursor:pointer}' +
      '[data-pb-path]:hover:not(:has([data-pb-path]:hover))>*{outline:2px dashed #6366f1;outline-offset:3px}'
    doc.head?.appendChild(style)
    doc.addEventListener('click', (e) => {
      const target = (e.target as Element | null)?.closest?.('[data-pb-path]')
      e.preventDefault() // links in the preview go nowhere
      const path = target?.getAttribute('data-pb-path')
      if (path) focusBuilderBlock('field-builder_payload', path)
    })
  }
  useEffect(() => {
    if (!open) return
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => {
      api
        .plugin<{ html: string }>('cms', '/preview', { method: 'POST', body: JSON.stringify({ payload: JSON.parse(json), width }) })
        .then((r) => {
          setHtml(r.html)
          setError(null)
        })
        .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
    }, 600)
    return () => window.clearTimeout(timer.current)
  }, [api, json, width, open])
  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)} className="rounded-lg bg-white px-3 py-1.5 text-sm font-semibold shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 dark:bg-white/5 dark:ring-white/10">
        {L('نمایش پیش‌نمایش زنده', 'Show live preview')}
      </button>
    )
  }
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2" role="group" aria-label={L('دستگاه', 'Device')}>
        {(Object.keys(devices) as (keyof typeof devices)[]).map((d) => (
          <button
            key={d}
            type="button"
            aria-pressed={device === d}
            onClick={() => setDevice(d)}
            className={'rounded-lg px-3 py-1 text-sm ring-1 ' + (device === d ? 'bg-primary-600 text-white ring-primary-600' : 'ring-gray-950/10 dark:ring-white/10')}
          >
            {devices[d].label}
          </button>
        ))}
        <button type="button" onClick={() => setOpen(false)} className="ms-auto text-sm text-gray-500 hover:text-gray-800">
          {L('بستن', 'Close')}
        </button>
      </div>
      {error && <p className="text-sm text-warning-600">{error}</p>}
      <p className="text-xs text-gray-500">{L('روی هر بلوک پیش‌نمایش بزنید تا همان بلوک در فرم باز شود.', 'Click a block in the preview to open it in the form.')}</p>
      <div className="overflow-hidden rounded-xl bg-gray-100 p-2 dark:bg-white/5">
        {/* Same origin so images load, but no scripts: the HTML can do nothing. Tabs show
            stacked here; the site switches them with its script. */}
        <iframe
          ref={frame}
          onLoad={wire}
          title={L('پیش‌نمایش زنده', 'Live preview')}
          sandbox="allow-same-origin"
          srcDoc={html}
          style={{ width: devices[device].w, maxWidth: '100%' }}
          className="mx-auto block h-[36rem] rounded-lg bg-white"
        />
      </div>
    </div>
  )
}
