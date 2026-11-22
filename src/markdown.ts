import { t } from './i18n'
// Toolbar edits for the Markdown editor, as a pure function of the text and selection.

export type MarkdownAction = 'bold' | 'italic' | 'code' | 'heading' | 'bullet' | 'number' | 'quote' | 'link'

export interface Edit {
  text: string
  start: number
  end: number
}

const wraps: Partial<Record<MarkdownAction, string>> = { bold: '**', italic: '_', code: '`' }
const prefixes: Partial<Record<MarkdownAction, (i: number) => string>> = {
  heading: () => '## ',
  bullet: () => '- ',
  number: (i) => `${i + 1}. `,
  quote: () => '> ',
}

export function applyMarkdown(text: string, start: number, end: number, action: MarkdownAction): Edit {
  const selected = text.slice(start, end)

  const wrap = wraps[action]
  if (wrap) {
    const inner = selected || t('markdown.text')
    const next = text.slice(0, start) + wrap + inner + wrap + text.slice(end)
    return { text: next, start: start + wrap.length, end: start + wrap.length + inner.length }
  }

  if (action === 'link') {
    const label = selected || t('markdown.linkText')
    const next = `${text.slice(0, start)}[${label}](https://)${text.slice(end)}`
    const urlStart = start + label.length + 3
    return { text: next, start: urlStart, end: urlStart + 'https://'.length }
  }

  // Line prefixes apply to every line the selection touches.
  const prefix = prefixes[action]!
  const lineStart = text.lastIndexOf('\n', start - 1) + 1
  const nextBreak = text.indexOf('\n', end)
  const lineEnd = nextBreak === -1 ? text.length : nextBreak
  const lines = text.slice(lineStart, lineEnd).split('\n')
  const changed = lines.map((line, i) => prefix(i) + line).join('\n')
  const next = text.slice(0, lineStart) + changed + text.slice(lineEnd)
  return { text: next, start: lineStart, end: lineStart + changed.length }
}
