import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { t } from '../i18n'

// Loaded lazily, only when the preview tab is opened.
// The HTML is sanitized: Markdown may contain raw HTML (e.g. <img onerror>), and the
// person previewing may not be the one who wrote it.
export default function MarkdownPreview({ markdown }: { markdown: string }) {
  const html = DOMPurify.sanitize(marked.parse(markdown, { async: false, gfm: true, breaks: true }))
  return (
    <div
      className="mks-prose min-h-40 px-3 py-2 text-sm text-gray-950 dark:text-white"
      dir="auto"
      // eslint-disable-next-line react/no-danger -- sanitized above
      dangerouslySetInnerHTML={{ __html: html || `<p class="text-gray-400">${t('markdown.nothing')}</p>` }}
    />
  )
}
