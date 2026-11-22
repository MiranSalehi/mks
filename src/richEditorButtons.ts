import type { ReactNode } from 'react'

/** What a toolbar button added by a plugin can do to the editor. */
export interface RichEditorHandle {
  /** Inserts HTML at the cursor; the editor keeps what its schema knows. */
  insertHTML(html: string): void
  /** Inserts plain text at the cursor. */
  insertText(text: string): void
}

/**
 * A button at the start of every rich editor's toolbar (MKSine's "Insert media" and
 * "Insert shortcode"). While it is open, `dialog` is shown; it calls the handle and close().
 */
export interface RichEditorButton {
  key: string
  label: string
  icon: ReactNode
  dialog: (props: { editor: RichEditorHandle; close: () => void }) => ReactNode
}

const buttons: RichEditorButton[] = []

/** Adds a button to the rich editors' toolbars (once per key). */
export function addRichEditorButton(button: RichEditorButton): void {
  if (!buttons.some((b) => b.key === button.key)) buttons.push(button)
}

export function richEditorButtons(): readonly RichEditorButton[] {
  return buttons
}
