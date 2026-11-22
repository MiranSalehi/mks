import { lazy, Suspense, useId, useRef, useState } from 'react'
import type { MarkdownEditor } from '../form'
import { applyMarkdown, type MarkdownAction } from '../markdown'
import { formatNumber, t } from '../i18n'

const MarkdownPreview = lazy(() => import('./markdownPreview'))

const tools: { action: MarkdownAction; label: string; content: string }[] = [
  { action: 'bold', label: t('editor.bold'), content: 'B' },
  { action: 'italic', label: t('editor.italic'), content: 'I' },
  { action: 'heading', label: t('editor.heading'), content: 'H' },
  { action: 'bullet', label: t('editor.bullet'), content: '•' },
  { action: 'number', label: t('editor.ordered'), content: formatNumber(1) + '.' },
  { action: 'quote', label: t('editor.quote'), content: '❝' },
  { action: 'code', label: t('editor.code'), content: '</>' },
  { action: 'link', label: t('editor.link'), content: '🔗' },
]

interface Props {
  id: string
  field: MarkdownEditor<any>
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  invalid?: boolean
  a11y: Record<string, unknown>
}

export function MarkdownEditorControl({ id, field, value, onChange, disabled, invalid, a11y }: Props) {
  const text = typeof value === 'string' ? value : ''
  const [preview, setPreview] = useState(false)
  const area = useRef<HTMLTextAreaElement>(null)
  const tabsId = useId()

  function run(action: MarkdownAction) {
    const el = area.current
    if (!el) return
    const edit = applyMarkdown(text, el.selectionStart, el.selectionEnd, action)
    onChange(edit.text)
    // Restore the selection after React updates the textarea.
    requestAnimationFrame(() => {
      el.focus()
      el.setSelectionRange(edit.start, edit.end)
    })
  }

  const tab = (isPreview: boolean, label: string) => (
    <button
      type="button"
      role="tab"
      id={`${tabsId}-${isPreview ? 'preview' : 'write'}`}
      aria-selected={preview === isPreview}
      onClick={() => setPreview(isPreview)}
      className={
        'rounded px-2 py-1 text-sm ' +
        (preview === isPreview ? 'bg-gray-200 font-medium dark:bg-white/15' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/10')
      }
    >
      {label}
    </button>
  )

  return (
    <div
      className={
        'rounded-lg bg-white shadow-sm ring-1 focus-within:ring-2 focus-within:ring-primary-600 dark:bg-white/5 ' +
        (invalid ? 'ring-danger-600 dark:ring-danger-400' : 'ring-gray-950/10 dark:ring-white/20') +
        (disabled ? ' opacity-60' : '')
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 p-1 dark:border-white/10">
        <div role="tablist" aria-label={t('markdown.mode')} className="flex gap-1">
          {tab(false, t('markdown.write'))}
          {tab(true, t('markdown.preview'))}
        </div>
        {!preview && !disabled && (
          <div role="toolbar" aria-label={t('markdown.toolbar')} className="flex flex-wrap gap-0.5">
            {tools.map((t) => (
              <button
                key={t.action}
                type="button"
                aria-label={t.label}
                title={t.label}
                onMouseDown={(e) => e.preventDefault()} // keep the selection in the textarea
                onClick={() => run(t.action)}
                className="min-w-8 rounded px-2 py-1 text-sm text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/10"
              >
                {t.content}
              </button>
            ))}
          </div>
        )}
      </div>
      {preview ? (
        <div role="tabpanel" aria-labelledby={`${tabsId}-preview`}>
          <Suspense fallback={<div className="min-h-40 animate-pulse" />}>
            <MarkdownPreview markdown={text} />
          </Suspense>
        </div>
      ) : (
        <textarea
          ref={area}
          id={id}
          rows={field._rows}
          dir="auto"
          disabled={disabled}
          value={text}
          onChange={(e) => onChange(e.target.value)}
          {...a11y}
          className="block w-full resize-y border-0 bg-transparent px-3 py-2 font-mono text-sm text-gray-950 outline-none dark:text-white"
        />
      )}
    </div>
  )
}
