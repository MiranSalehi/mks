import DOMPurify from 'dompurify'
import { marked } from 'marked'

// Loaded lazily, only by pages that show rich text. The HTML is sanitized even though
// the server cleans rich text on save: the viewer may not be the one who wrote it, and
// Markdown may hold raw HTML.
export default function RichContent({ html, markdown }: { html?: string; markdown?: string }) {
  const source = markdown != null ? marked.parse(markdown, { async: false, gfm: true, breaks: true }) : (html ?? '')
  return (
    <div
      className="mks-prose text-sm leading-7 text-gray-800 dark:text-gray-200"
      dir="auto"
      // eslint-disable-next-line react/no-danger -- sanitized here
      dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(source) }}
    />
  )
}
