import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { defaultFileUrl, defineField, tr, type CustomFieldProps } from '../../index'

// MKSine's image editor: crop (free, 16:9, 4:3, 1:1), rotate and mirror a library image.
// The value is what the server applies, in this order: rotate (clockwise), flip, then
// crop in pixels of the rotated image. The server works on the image as browsers show it
// (EXIF orientation applied), like the canvas here.

export interface ImageEditValue {
  rotate: number
  flip: boolean
  crop?: { x: number; y: number; width: number; height: number }
}

interface Box {
  x: number
  y: number
  w: number
  h: number
}

const ratios: { label: string; value: number | null }[] = [
  { label: tr({ fa: 'آزاد', en: 'Free' }), value: null },
  { label: '16:9', value: 16 / 9 },
  { label: '4:3', value: 4 / 3 },
  { label: '1:1', value: 1 },
]

const MAX_W = 640
const MAX_H = 420

const tool =
  'rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-gray-950/10 hover:bg-gray-50 dark:ring-white/10 dark:hover:bg-white/5'

function ImageEditorControl({ id, value, onChange, record }: CustomFieldProps<ImageEditValue | null, Record<string, never>>) {
  const edit: ImageEditValue = value ?? { rotate: 0, flip: false }
  const canvas = useRef<HTMLCanvasElement>(null)
  const [img, setImg] = useState<HTMLImageElement | null>(null)
  const [failed, setFailed] = useState(false)
  const [ratio, setRatio] = useState<number | null>(null)
  const [box, setBox] = useState<Box | null>(null)
  const drag = useRef<{ mode: 'move' | 'draw' | 'resize'; startX: number; startY: number; box: Box } | null>(null)
  // The room the editor has (a modal can be narrow).
  const wrap = useRef<HTMLDivElement>(null)
  const [room, setRoom] = useState(MAX_W)
  useEffect(() => {
    const el = wrap.current
    if (!el) return
    const measure = () => setRoom(Math.max(200, Math.min(MAX_W, el.clientWidth)))
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const path = typeof record?.path === 'string' ? record.path : ''
  const version = typeof record?.updated_at === 'string' ? record.updated_at : ''

  useEffect(() => {
    if (!path) return
    const image = new Image()
    image.onload = () => setImg(image)
    image.onerror = () => setFailed(true)
    image.src = defaultFileUrl(path) + (version ? `?v=${encodeURIComponent(version)}` : '')
  }, [path, version])

  // The rotated image's natural size, and the scale of the canvas on screen.
  const turned = edit.rotate % 180 !== 0
  const natW = img ? (turned ? img.naturalHeight : img.naturalWidth) : 0
  const natH = img ? (turned ? img.naturalWidth : img.naturalHeight) : 0
  const scale = natW && natH ? Math.min(1, room / natW, MAX_H / natH) : 1
  const viewW = Math.round(natW * scale)
  const viewH = Math.round(natH * scale)

  // When the scale changes (the room changed), draw the crop again from the value.
  const crop = edit.crop
  useEffect(() => {
    if (!drag.current) setBox(crop ? { x: crop.x * scale, y: crop.y * scale, w: crop.width * scale, h: crop.height * scale } : null)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only on a new scale
  }, [scale])

  useEffect(() => {
    const c = canvas.current
    if (!c || !img) return
    c.width = viewW
    c.height = viewH
    const g = c.getContext('2d')
    if (!g) return
    g.save()
    // Like the server: turn the picture, then mirror the turned picture left to right.
    g.translate(viewW / 2, viewH / 2)
    if (edit.flip) g.scale(-1, 1)
    g.rotate((edit.rotate * Math.PI) / 180)
    const w = img.naturalWidth * scale
    const h = img.naturalHeight * scale
    g.drawImage(img, -w / 2, -h / 2, w, h)
    g.restore()
  }, [img, edit.rotate, edit.flip, viewW, viewH, scale])

  // The crop box (screen pixels) goes to the value in image pixels.
  function commit(next: Box | null, e: ImageEditValue = edit) {
    setBox(next)
    if (!next || next.w < 4 || next.h < 4) {
      onChange({ rotate: e.rotate, flip: e.flip })
      return
    }
    onChange({
      rotate: e.rotate,
      flip: e.flip,
      crop: {
        x: Math.round(next.x / scale),
        y: Math.round(next.y / scale),
        width: Math.round(next.w / scale),
        height: Math.round(next.h / scale),
      },
    })
  }

  function centered(r: number | null): Box | null {
    if (r === null) return null
    let w = viewW
    let h = w / r
    if (h > viewH) {
      h = viewH
      w = h * r
    }
    return { x: (viewW - w) / 2, y: (viewH - h) / 2, w, h }
  }

  function chooseRatio(r: number | null) {
    setRatio(r)
    commit(centered(r))
  }

  function transform(next: Partial<ImageEditValue>) {
    const e = { ...edit, ...next }
    // The picture changed shape: start the crop again.
    setBox(null)
    setRatio(null)
    onChange({ rotate: e.rotate, flip: e.flip })
  }

  const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))

  function point(e: ReactPointerEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    return { x: clamp(e.clientX - r.left, 0, viewW), y: clamp(e.clientY - r.top, 0, viewH) }
  }

  function onDown(e: ReactPointerEvent<HTMLDivElement>) {
    const p = point(e)
    const target = e.target as HTMLElement
    e.currentTarget.setPointerCapture(e.pointerId)
    if (box && target.dataset.handle) drag.current = { mode: 'resize', startX: p.x, startY: p.y, box }
    else if (box && p.x >= box.x && p.x <= box.x + box.w && p.y >= box.y && p.y <= box.y + box.h) drag.current = { mode: 'move', startX: p.x, startY: p.y, box }
    else drag.current = { mode: 'draw', startX: p.x, startY: p.y, box: { x: p.x, y: p.y, w: 0, h: 0 } }
  }

  function onMove(e: ReactPointerEvent<HTMLDivElement>) {
    const d = drag.current
    if (!d) return
    const p = point(e)
    const dx = p.x - d.startX
    const dy = p.y - d.startY
    if (d.mode === 'move') {
      setBox({ ...d.box, x: clamp(d.box.x + dx, 0, viewW - d.box.w), y: clamp(d.box.y + dy, 0, viewH - d.box.h) })
      return
    }
    // Drawing or resizing: from a fixed corner to the pointer, keeping the ratio.
    const ox = d.mode === 'draw' ? d.startX : d.box.x
    const oy = d.mode === 'draw' ? d.startY : d.box.y
    let w = d.mode === 'draw' ? Math.abs(dx) : clamp(d.box.w + dx, 8, viewW - ox)
    let h = d.mode === 'draw' ? Math.abs(dy) : clamp(d.box.h + dy, 8, viewH - oy)
    if (ratio) h = w / ratio
    let x = d.mode === 'draw' && dx < 0 ? ox - w : ox
    let y = d.mode === 'draw' && dy < 0 ? oy - h : oy
    if (x < 0) {
      w += x
      x = 0
    }
    if (y < 0) {
      h += y
      y = 0
    }
    w = Math.min(w, viewW - x)
    h = Math.min(h, viewH - y)
    if (ratio) {
      // Shrink to fit inside while keeping the ratio.
      if (w / h > ratio) w = h * ratio
      else h = w / ratio
    }
    setBox({ x, y, w, h })
  }

  function onUp() {
    if (!drag.current) return
    drag.current = null
    commit(box)
  }

  if (!path) return <p className="text-sm text-gray-500">{tr({ fa: 'تصویری انتخاب نشده است.', en: 'No image.' })}</p>
  if (failed) return <p className="text-sm text-danger-600">{tr({ fa: 'تصویر بارگذاری نشد.', en: 'The image did not load.' })}</p>

  return (
    <div id={id} ref={wrap} className="space-y-3" dir="ltr">
      <div role="toolbar" aria-label={tr({ fa: 'ابزار تصویر', en: 'Image tools' })} className="flex flex-wrap items-center gap-1.5">
        {ratios.map((r) => (
          <button key={r.label} type="button" aria-pressed={ratio === r.value} onClick={() => chooseRatio(r.value)}
            className={tool + (ratio === r.value ? ' bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300' : '')}>
            {r.label}
          </button>
        ))}
        <span className="mx-1 h-4 w-px bg-gray-200 dark:bg-white/10" aria-hidden="true" />
        <button type="button" className={tool} onClick={() => transform({ rotate: (edit.rotate + 270) % 360 })} aria-label={tr({ fa: 'چرخش به چپ', en: 'Rotate left' })}>
          ⟲
        </button>
        <button type="button" className={tool} onClick={() => transform({ rotate: (edit.rotate + 90) % 360 })} aria-label={tr({ fa: 'چرخش به راست', en: 'Rotate right' })}>
          ⟳
        </button>
        <button type="button" className={tool} aria-pressed={edit.flip} onClick={() => transform({ flip: !edit.flip })} aria-label={tr({ fa: 'قرینه', en: 'Mirror' })}>
          ⇋
        </button>
        <button type="button" className={tool} onClick={() => { setRatio(null); commit(null) }}>
          {tr({ fa: 'بدون برش', en: 'No crop' })}
        </button>
        {edit.crop && (
          <span className="ms-1 text-xs text-gray-500" role="status">
            {edit.crop.width} × {edit.crop.height}
          </span>
        )}
      </div>
      {!img ? (
        <div className="h-48 animate-pulse rounded-lg bg-gray-100 dark:bg-white/5" />
      ) : (
        <div
          className="relative touch-none select-none overflow-hidden rounded-lg ring-1 ring-gray-950/10 dark:ring-white/10"
          style={{ width: viewW, height: viewH, cursor: 'crosshair' }}
          onPointerDown={onDown}
          onPointerMove={onMove}
          onPointerUp={onUp}
          onPointerCancel={onUp}
        >
          <canvas ref={canvas} className="block" aria-label={tr({ fa: 'پیش‌نمایش تصویر', en: 'Image preview' })} />
          {box && box.w > 0 && box.h > 0 && (
            <div
              className="absolute cursor-move border-2 border-white"
              style={{ left: box.x, top: box.y, width: box.w, height: box.h, boxShadow: '0 0 0 9999px rgba(0,0,0,.5)' }}
            >
              <span data-handle="1" className="absolute -right-1.5 -bottom-1.5 size-3 cursor-nwse-resize rounded-sm border-2 border-white bg-primary-600" />
            </div>
          )}
        </div>
      )}
      <p className="text-xs text-gray-500" dir="auto">
        {tr({ fa: 'برای برش، روی تصویر بکشید؛ کادر را جابه‌جا کنید یا از گوشه‌اش اندازه دهید.', en: 'Drag on the image to crop; move the box or resize it from its corner.' })}
      </p>
    </div>
  )
}

/** The image editor field of the media library's "edit image" action. */
export const imageEditor = defineField<ImageEditValue | null, Record<string, never>>({ component: ImageEditorControl, defaults: {}, empty: null })
