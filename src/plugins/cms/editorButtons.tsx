import { addRichEditorButton, defaultFileUrl, Modal, tr, usePanel, type ModalHandle, type RichEditorHandle } from '../../index'
import { useRef } from 'react'
import { kindOf, mediaName, MediaPickerDialog } from './mediaPickerDialog'
import type { Media } from './types'

// MKSine's two buttons at the start of the content editor's toolbar: "Insert Media" opens
// the media library (several files: images, video, audio) and "Insert shortcode" lists the
// site's shortcodes.

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')

/** The HTML for library files, as MKSine's editor inserts them. */
export function mediaHTML(list: Media[]): string {
  return list
    .map((m) => {
      const url = esc(defaultFileUrl(m.path))
      switch (kindOf(m.mime_type)) {
        case 'image':
          return `<img src="${url}" alt="${esc(m.alt || mediaName(m))}">`
        case 'video':
          return `<video src="${url}"></video>`
        case 'audio':
          return `<audio src="${url}"></audio>`
        default:
          return `<p><a href="${url}">${esc(mediaName(m))}</a></p>`
      }
    })
    .join('')
}

interface ShortcodeInfo {
  tag: string
  label: string
  description?: string
  example: string
}

function ShortcodeDialog({ editor, close }: { editor: RichEditorHandle; close: () => void }) {
  const { meta } = usePanel()
  const modal = useRef<ModalHandle>(null)
  const catalog = (meta.extra?.cms_shortcodes as ShortcodeInfo[] | undefined) ?? []
  return (
    <Modal ref={modal} title={tr({ fa: 'درج شورتکد', en: 'Insert shortcode' })} size="md" onClose={close}>
      {catalog.length === 0 ? (
        <p className="text-sm text-gray-500 dark:text-gray-400">{tr({ fa: 'هنوز شورتکدی ثبت نشده است.', en: 'No shortcodes registered.' })}</p>
      ) : (
        <ul className="-mx-2 space-y-1">
          {catalog.map((s) => (
            <li key={s.tag}>
              <button
                type="button"
                onClick={() => {
                  editor.insertText(s.example)
                  modal.current?.close()
                }}
                className="flex w-full flex-col items-start gap-1 rounded-lg px-3 py-2.5 text-start hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none dark:hover:bg-white/5 dark:focus-visible:bg-white/5"
              >
                <span className="text-sm font-semibold text-gray-950 dark:text-white">{s.label}</span>
                {s.description && <span className="text-xs text-gray-500 dark:text-gray-400">{s.description}</span>}
                <code dir="ltr" className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-700 dark:bg-gray-900 dark:text-gray-300">
                  {s.example}
                </code>
              </button>
            </li>
          ))}
        </ul>
      )}
    </Modal>
  )
}

export function registerEditorButtons() {
  addRichEditorButton({
    key: 'insertMedia',
    label: 'Insert Media',
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="20" height="20" viewBox="0 0 24 24" aria-hidden="true" className="shrink-0">
        <path d="M19,13a1,1,0,0,0-1,1v.38L16.52,12.9a2.79,2.79,0,0,0-3.93,0l-.7.7L9.41,11.12a2.85,2.85,0,0,0-3.93,0L4,12.6V7A1,1,0,0,1,5,6h7a1,1,0,0,0,0-2H5A3,3,0,0,0,2,7V19a3,3,0,0,0,3,3H17a3,3,0,0,0,3-3V14A1,1,0,0,0,19,13ZM5,20a1,1,0,0,1-1-1V15.43l2.9-2.9a.79.79,0,0,1,1.09,0l3.17,3.17,0,0L15.46,20Zm13-1a.89.89,0,0,1-.18.53L13.31,15l.7-.7a.77.77,0,0,1,1.1,0L18,17.21ZM22.71,4.29l-3-3a1,1,0,0,0-.33-.21,1,1,0,0,0-.76,0,1,1,0,0,0-.33.21l-3,3a1,1,0,0,0,1.42,1.42L18,4.41V10a1,1,0,0,0,2,0V4.41l1.29,1.3a1,1,0,0,0,1.42,0A1,1,0,0,0,22.71,4.29Z" />
      </svg>
    ),
    dialog: ({ editor, close }) => (
      <MediaPickerDialog multiple accept="any" selected={[]} onConfirm={(list) => editor.insertHTML(mediaHTML(list))} onClose={close} />
    ),
  })
  addRichEditorButton({
    key: 'insertShortcode',
    label: tr({ fa: 'درج شورتکد', en: 'Insert shortcode' }),
    icon: (
      <svg viewBox="0 0 20 20" width="20" height="20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="shrink-0">
        <path d="M3 6h3v8H3V6zm11 0h3v8h-3V6zM8 7h4v6H8V7z" fill="currentColor" />
      </svg>
    ),
    dialog: (props) => <ShortcodeDialog {...props} />,
  })
}
