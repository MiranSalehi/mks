import { useEffect, useRef } from 'react'
import { basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { EditorView, keymap, placeholder as placeholderExt } from '@codemirror/view'
import { indentWithTab } from '@codemirror/commands'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { css } from '@codemirror/lang-css'
import { javascript } from '@codemirror/lang-javascript'
import { tags } from '@lezer/highlight'

export type CodeLanguage = 'css' | 'javascript'

export interface CodeEditorProps {
  value: string
  onChange: (value: string) => void
  language: CodeLanguage
  /** For screen readers (the visible label is in the page). */
  label?: string
  disabled?: boolean
  /** Grey text shown while the editor is empty. */
  placeholder?: string
  /** Height of the editing surface, in pixels. */
  height?: number
}

// Colors come from CSS variables so the same editor follows the panel's light/dark mode.
const codeHighlight = HighlightStyle.define([
  { tag: tags.comment, color: 'var(--mks-code-comment, #64748b)', fontStyle: 'italic' },
  { tag: [tags.keyword, tags.modifier, tags.operatorKeyword], color: 'var(--mks-code-keyword, #7c3aed)' },
  { tag: [tags.string, tags.special(tags.string), tags.regexp], color: 'var(--mks-code-string, #047857)' },
  { tag: [tags.number, tags.bool, tags.null], color: 'var(--mks-code-number, #b45309)' },
  { tag: [tags.propertyName, tags.attributeName], color: 'var(--mks-code-property, #1d4ed8)' },
  { tag: [tags.variableName, tags.definition(tags.variableName)], color: 'var(--mks-code-variable, #0f172a)' },
  { tag: [tags.typeName, tags.className, tags.tagName, tags.namespace], color: 'var(--mks-code-type, #0e7490)' },
  { tag: [tags.function(tags.variableName), tags.function(tags.propertyName)], color: 'var(--mks-code-function, #c2410c)' },
  { tag: [tags.punctuation, tags.bracket, tags.separator], color: 'var(--mks-code-punctuation, #475569)' },
  { tag: tags.invalid, color: '#dc2626' },
])

const codeTheme = EditorView.theme({
  '&': { height: '100%', fontSize: '13px', backgroundColor: 'transparent', color: 'inherit' },
  '.cm-scroller': {
    overflow: 'auto',
    lineHeight: '1.65',
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace',
  },
  '.cm-content': { padding: '10px 0', caretColor: 'currentColor' },
  '.cm-gutters': { backgroundColor: 'transparent', border: 'none', color: 'var(--mks-code-gutter, #94a3b8)' },
  '.cm-activeLine': { backgroundColor: 'var(--mks-code-active-line, rgb(15 23 42 / 0.04))' },
  '.cm-activeLineGutter': { backgroundColor: 'transparent' },
  '&.cm-focused': { outline: 'none' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, .cm-content ::selection': {
    backgroundColor: 'var(--mks-code-selection, rgb(59 130 246 / 0.18))',
  },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'currentColor' },
})

/**
 * A small code editor for CSS and JavaScript (CodeMirror 6): line numbers, bracket
 * matching, history, search, Tab indentation and a light/dark highlight style.
 *
 *	<CodeEditor value={css} onChange={setCss} language="css" label="Custom CSS" />
 */
export function CodeEditor({ value, onChange, language, label, disabled, placeholder, height = 240 }: CodeEditorProps) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const change = useRef(onChange)
  const latest = useRef(value)
  change.current = onChange
  latest.current = value

  useEffect(() => {
    if (!host.current) return
    const editor = new EditorView({
      parent: host.current,
      state: EditorState.create({
        doc: latest.current,
        extensions: [
          basicSetup,
          keymap.of([indentWithTab]),
          language === 'css' ? css() : javascript(),
          codeTheme,
          EditorView.lineWrapping,
          syntaxHighlighting(codeHighlight),
          placeholderExt(placeholder ?? ''),
          EditorView.contentAttributes.of({
            'aria-label': label ?? language,
            dir: 'ltr',
            spellcheck: 'false',
            autocapitalize: 'off',
            autocomplete: 'off',
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) change.current(update.state.doc.toString())
          }),
          EditorState.readOnly.of(!!disabled),
          EditorView.editable.of(!disabled),
        ],
      }),
    })
    view.current = editor
    return () => {
      editor.destroy()
      view.current = null
    }
  }, [language, label, disabled, placeholder])

  // Another component changed the value (a reset, or a value loaded after mount).
  useEffect(() => {
    const editor = view.current
    if (!editor) return
    const current = editor.state.doc.toString()
    if (current !== value) editor.dispatch({ changes: { from: 0, to: current.length, insert: value } })
  }, [value])

  return (
    <div
      dir="ltr"
      className={
        'mks-code-editor overflow-hidden rounded-lg bg-white ring-1 ring-gray-950/10 focus-within:ring-2 focus-within:ring-primary-600 dark:bg-white/5 dark:ring-white/10 ' +
        (disabled ? 'opacity-60' : '')
      }
    >
      <div ref={host} style={{ height }} />
    </div>
  )
}
