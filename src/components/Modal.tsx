import { useEffect, useId, useImperativeHandle, useRef, type ReactNode, type Ref } from 'react'
import { CloseIcon, WarningIcon } from './icons'
import { t } from '../i18n'

export interface ModalHandle {
  /** Closes with the exit animation, then calls onClose. */
  close: () => void
}

const EXIT_MS = 240

const widths = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
}

export interface ModalProps {
  ref?: Ref<ModalHandle>
  title: string
  description?: ReactNode
  /** A danger dialog (e.g. delete) shows a warning icon. */
  tone?: 'default' | 'danger'
  size?: keyof typeof widths
  /** Escape, the close button and (if closeOnBackdrop) a click outside close the dialog. */
  dismissible?: boolean
  /** Off for forms, so a stray click outside does not throw away what was typed. */
  closeOnBackdrop?: boolean
  /** Called after the exit animation: remove the modal here. */
  onClose: () => void
  children?: ReactNode
  footer?: ReactNode
}

/**
 * A native <dialog> (focus trap, Escape and top layer for free) that fades and rises in,
 * and out again before it is removed. The animation is CSS (styles.css: .mks-dialog).
 */
export function Modal({
  ref,
  title,
  description,
  tone = 'default',
  size = 'md',
  dismissible = true,
  closeOnBackdrop = true,
  onClose,
  children,
  footer,
}: ModalProps) {
  const dialog = useRef<HTMLDialogElement>(null)
  const closing = useRef(false)
  const titleId = useId()
  const descriptionId = useId()
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  function close() {
    const d = dialog.current
    if (!d || closing.current) return
    closing.current = true
    d.close() // starts the exit transition
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    window.setTimeout(() => onCloseRef.current(), reduced ? 0 : EXIT_MS)
  }
  useImperativeHandle(ref, () => ({ close }))

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      onCancel={(e) => {
        e.preventDefault()
        if (dismissible) close()
      }}
      // A click on the dialog element itself is a click on the backdrop around the panel.
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && dismissible && closeOnBackdrop) close()
      }}
      className={
        'mks-dialog m-auto max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] overflow-visible bg-transparent p-0 text-gray-950 dark:text-white ' +
        widths[size]
      }
    >
      <div className="flex max-h-[calc(100dvh-2rem)] flex-col rounded-2xl bg-white shadow-xl ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10">
        <header className="flex items-start gap-4 px-6 pt-6 pb-5">
          {tone === 'danger' && (
            <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-danger-50 text-danger-600 ring-8 ring-danger-50/50 dark:bg-danger-500/10 dark:text-danger-400 dark:ring-danger-500/5">
              <WarningIcon />
            </span>
          )}
          <div className="min-w-0 flex-1 space-y-1.5 pt-0.5">
            <h2 id={titleId} className="text-lg font-semibold tracking-tight">
              {title}
            </h2>
            {description && (
              <div id={descriptionId} className="text-sm leading-6 text-gray-600 dark:text-gray-400">
                {description}
              </div>
            )}
          </div>
          {dismissible && (
            <button
              type="button"
              onClick={close}
              aria-label={t('common.close')}
              className="-me-2 -mt-1 rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-white/5 dark:hover:text-gray-200"
            >
              <CloseIcon />
            </button>
          )}
        </header>
        {/* The body scrolls when it is long; small dialogs let dropdowns reach past the edge. */}
        <div className={'px-6 pb-6 has-[>form:empty]:hidden ' + (size === 'lg' || size === 'xl' ? 'overflow-y-auto' : '')}>{children}</div>
        {footer && (
          <footer className="flex flex-wrap items-center gap-3 rounded-b-2xl border-t border-gray-100 bg-gray-50/60 px-6 py-4 dark:border-white/5 dark:bg-white/[0.02]">
            {footer}
          </footer>
        )}
      </div>
    </dialog>
  )
}
