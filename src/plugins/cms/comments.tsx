import { defineResource, tr } from '../../index'
import type { Comment } from './types'

export const commentStatusLabels = {
  pending: tr({ fa: 'در انتظار', en: 'Pending' }),
  approved: tr({ fa: 'تأییدشده', en: 'Approved' }),
  spam: tr({ fa: 'هرزنامه', en: 'Spam' }),
  trash: tr({ fa: 'زباله‌دان', en: 'Trash' }),
}

const ChatIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M21 12a8 8 0 0 1-11.6 7.1L4 20l1-4.6A8 8 0 1 1 21 12z" />
  </svg>
)

const stars = (n: unknown) => (typeof n === 'number' && n >= 1 && n <= 5 ? '★'.repeat(n) + '☆'.repeat(5 - n) : '')

/** Readers' comments: moderated here, answered with the reply action. */
export const comments = defineResource<Comment>({
  name: 'cms_comments',
  label: tr({ fa: 'دیدگاه', en: 'Comment' }),
  pluralLabel: tr({ fa: 'دیدگاه‌ها', en: 'Comments' }),
  navigationSort: 4,
  icon: <ChatIcon />,
  group: tr({ fa: 'محتوا', en: 'Content' }),
  defaultSort: '-created_at',
  recordTitle: (c) => c.author_name,

  table: (t) => [
    t.text('author_name').label(tr({ fa: 'نویسنده', en: 'Author' })).searchable().sortable().weight('medium')
      .description((c) => c.author_email ?? ''),
    t.text('content').label(tr({ fa: 'دیدگاه', en: 'Comment' })).searchable().limit(80),
    t.text('post.title').label(tr({ fa: 'برای', en: 'On' }))
      .format((_, c) => c.post?.title ?? c.page?.title ?? ''),
    t.text('rating').label(tr({ fa: 'امتیاز', en: 'Rating' })).sortable().format((v) => stars(v)),
    t.badge('status').label(tr({ fa: 'وضعیت', en: 'Status' })).sortable()
      .colors({ pending: 'warning', approved: 'success', spam: 'danger', trash: 'gray' }).labels(commentStatusLabels),
    t.text('parent_id').label(tr({ fa: 'نوع', en: 'Type' })).format((v) => (v ? tr({ fa: 'پاسخ', en: 'Reply' }) : tr({ fa: 'اصلی', en: 'Root' }))).toggleable(true),
    t.date('created_at').label(tr({ fa: 'زمان', en: 'Time' })).since().sortable(),
  ],

  // MKSine's list tabs: all, then one per status.
  tabs: {
    filter: 'status',
    tabs: [{ label: tr({ fa: 'همه', en: 'All' }) }, ...Object.entries(commentStatusLabels).map(([value, label]) => ({ label, value }))],
  },

  filters: (f) => [
    f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(commentStatusLabels).multiple(),
    f.select('rating').label(tr({ fa: 'امتیاز', en: 'Rating' })).options({ 5: '★★★★★', 4: '★★★★', 3: '★★★', 2: '★★', 1: '★' }),
    f.ternary('parent_id').label(tr({ fa: 'نوع', en: 'Type' })).nullable().labels(tr({ fa: 'پاسخ', en: 'Reply' }), tr({ fa: 'اصلی', en: 'Root' })),
  ],

  actions: (a) => [
    a.make('approve').label(tr({ fa: 'تأیید', en: 'Approve' })).color('success').row().bulk(),
    a.make('reply').label(tr({ fa: 'پاسخ', en: 'Reply' })).color('primary').row()
      .form((f) => [f.textarea('content').label(tr({ fa: 'متن پاسخ', en: 'Reply' })).rows(4).required().columnSpan('full')])
      .modalSubmitLabel(tr({ fa: 'انتشار پاسخ', en: 'Publish reply' })),
    a.make('spam').label(tr({ fa: 'هرزنامه', en: 'Spam' })).color('danger').row().bulk()
      .requiresConfirmation({ description: tr({ fa: 'دیدگاه هرزنامه علامت می‌خورد و در سایت دیده نمی‌شود.', en: 'The comment is marked as spam and hidden from the site.' }) }),
    a.make('trash').label(tr({ fa: 'زباله‌دان', en: 'Trash' })).color('gray').row().bulk()
      .requiresConfirmation({ description: tr({ fa: 'دیدگاه به زباله‌دان می‌رود و در سایت دیده نمی‌شود.', en: 'The comment goes to the trash and is hidden from the site.' }) }),
    a.make('restore').label(tr({ fa: 'برگرداندن به انتظار', en: 'Back to pending' })).color('gray').row(),
  ],

  infolist: (i) => [
    i.section(tr({ fa: 'دیدگاه', en: 'Comment' })).schema([
      i.text('author_name').label(tr({ fa: 'نویسنده', en: 'Author' })),
      i.text('author_email').label(tr({ fa: 'ایمیل', en: 'Email' })).copyable(),
      i.badge('status').label(tr({ fa: 'وضعیت', en: 'Status' })).labels(commentStatusLabels),
      i.text('rating').label(tr({ fa: 'امتیاز', en: 'Rating' })).format((v) => stars(v)),
      i.text('content').label(tr({ fa: 'متن', en: 'Text' })).columnSpan('full'),
      i.text('ip_address').label(tr({ fa: 'نشانی IP', en: 'IP address' })),
      i.text('user_agent').label(tr({ fa: 'مرورگر', en: 'Browser' })).columnSpan('full'),
    ]),
  ],

  form: (f) => [
    f.textInput('author_name').label(tr({ fa: 'نام', en: 'Name' })).required().maxLength(255),
    f.textInput('author_email').label(tr({ fa: 'ایمیل', en: 'Email' })).email().dir('ltr'),
    f.select('rating').label(tr({ fa: 'امتیاز', en: 'Rating' })).options({ 5: '5', 4: '4', 3: '3', 2: '2', 1: '1' }),
    f.select('status').label(tr({ fa: 'وضعیت', en: 'Status' })).options(commentStatusLabels).required(),
    f.textarea('content').label(tr({ fa: 'متن', en: 'Text' })).rows(5).required().columnSpan('full'),
  ],
})
