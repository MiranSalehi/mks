import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from 'react'
import { isWithin, moveBlock, type Place } from '../builderMove'
import type { Block, Builder, FieldContext, Repeater } from '../form'
import { applyChange, initialValues } from '../formState'
import type { FieldErrors, ResourceRecord } from '../protocol'
import { ChevronIcon, CloseIcon, CopyIcon } from './icons'
import { Schema } from './Schema'
import { direction, numberFormat, t } from '../i18n'
import { indentItem, outdentItem } from '../treeOps'

const fa = numberFormat()

interface Base {
  id: string
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  /** Errors for paths inside this field, e.g. "faqs.0.question". */
  errors: FieldErrors
  ctx: FieldContext<any>
  name: string
  /** Id prefix of this field, e.g. "faqs-"; items add their index. */
  idPrefix: string
}

/** Stable React keys for list items, kept in step with moves and deletes. */
function useItemKeys(length: number) {
  const next = useRef(0)
  const [keys, setKeys] = useState<number[]>(() => Array.from({ length }, () => next.current++))
  // A value replaced from outside (e.g. a reset) gets fresh keys.
  useEffect(() => {
    if (keys.length !== length) setKeys(Array.from({ length }, () => next.current++))
  }, [length, keys.length])
  return {
    keys: keys.length === length ? keys : Array.from({ length }, (_, i) => keys[i] ?? -1 - i),
    add: () => setKeys((k) => [...k, next.current++]),
    remove: (i: number) => setKeys((k) => k.filter((_, j) => j !== i)),
    move: (i: number, j: number) =>
      setKeys((k) => {
        const copy = [...k]
        ;[copy[i], copy[j]] = [copy[j], copy[i]]
        return copy
      }),
  }
}

/** Errors under "name.index." with that prefix removed. */
function itemErrors(errors: FieldErrors, prefix: string): FieldErrors {
  const out: FieldErrors = {}
  for (const [k, v] of Object.entries(errors)) if (k.startsWith(prefix)) out[k.slice(prefix.length)] = v
  return out
}

/** Inside an item, get() sees the item's own fields first, then the rest of the form. */
function itemContext(item: ResourceRecord, parent: FieldContext<any>): FieldContext<any> {
  return { ...parent, get: (n) => (n in item ? item[n] : parent.get(n)) }
}

/** A callback that applies a computed tree, or none when the move is not possible. */
function optional(next: ResourceRecord[] | null, apply: (t: ResourceRecord[]) => void): (() => void) | undefined {
  return next ? () => apply(next) : undefined
}

function move<T>(list: T[], i: number, j: number): T[] {
  const copy = [...list]
  ;[copy[i], copy[j]] = [copy[j], copy[i]]
  return copy
}

/** Inside an item of a tree repeater: the whole tree and the item's place in it. */
interface TreeItem {
  root: ResourceRecord[]
  setRoot: (tree: ResourceRecord[]) => void
  path: number[]
  children: string
}
const TreeItemContext = createContext<TreeItem | null>(null)

export function RepeaterControl({ id, field, value, onChange, disabled, errors, ctx, name, idPrefix }: Base & { field: Repeater<any> }) {
  const items = Array.isArray(value) ? (value as ResourceRecord[]) : []
  const keys = useItemKeys(items.length)
  const canAdd = !disabled && (field._maxItems === undefined || items.length < field._maxItems)
  // A tree: the top list holds the tree; a nested list is the children of an item.
  const outer = useContext(TreeItemContext)
  const tree = field._tree
  const nested = tree && outer && outer.children === tree.children && name === tree.children ? outer : null
  const root = tree ? (nested ? nested.root : items) : []
  const setRoot = tree ? (nested ? nested.setRoot : (t: ResourceRecord[]) => onChange(t)) : () => {}
  const listPath = nested ? nested.path : []

  return (
    <div id={id} role="group" className="space-y-3">
      {items.map((item, i) => (
        <ItemCard
          key={keys.keys[i]}
          marker={{ 'data-pb-slot': String(i) }}
          title={field._itemLabel?.(item, i) || `${fa.format(i + 1)}`}
          index={i}
          count={items.length}
          disabled={disabled}
          canRemove={items.length > field._minItems}
          reorderable={field._reorderable}
          onMove={(j) => {
            keys.move(i, j)
            onChange(move(items, i, j))
          }}
          onRemove={() => {
            keys.remove(i)
            onChange(items.filter((_, j) => j !== i))
          }}
          onIndent={tree ? optional(indentItem(root, [...listPath, i], tree.children, tree.maxDepth), setRoot) : undefined}
          onOutdent={tree ? optional(outdentItem(root, [...listPath, i], tree.children), setRoot) : undefined}
        >
          <TreeItemContext.Provider value={tree ? { root, setRoot, path: [...listPath, i], children: tree.children } : null}>
          <SlotContext.Provider value={i}>
          <Schema
            nodes={field._schema}
            columns={field._columns}
            ctx={itemContext(item, ctx)}
            values={item}
            errors={itemErrors(errors, `${name}.${i}.`)}
            idPrefix={`${idPrefix}${keys.keys[i]}-`}
            onChange={(child, v) => {
              // Hooks of fields inside the item run too; set() works within the item.
              const r = applyChange(field._fields, item, child, v, ctx.operation, ctx.record)
              onChange(items.map((it, j) => (j === i ? r.values : it)))
            }}
          />
          </SlotContext.Provider>
          </TreeItemContext.Provider>
        </ItemCard>
      ))}
      {items.length === 0 && <p className="text-sm text-gray-500 dark:text-gray-400">{t('repeater.empty')}</p>}
      {canAdd && (
        <button
          type="button"
          onClick={() => {
            keys.add()
            onChange([...items, initialValues(field._fields)])
          }}
          className="rounded-lg bg-white px-3 py-1.5 text-sm font-semibold text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 hover:ring-gray-950/20 active:scale-[0.98] dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10"
        >
          + {field._addLabel}
        </button>
      )}
    </div>
  )
}

interface BuilderItem {
  type: string
  data: ResourceRecord
}

/**
 * Undo and redo for a value: changes less than a moment apart (typing) are one step.
 * Restoring a step goes through onChange like any edit.
 */
function useHistory(enabled: boolean, value: unknown, onChange: (value: unknown) => void) {
  const past = useRef<string[]>([])
  const future = useRef<string[]>([])
  const current = useRef(JSON.stringify(value ?? []))
  const lastAt = useRef(0)
  const restoring = useRef(false)
  const [, rerender] = useState(0)
  const json = JSON.stringify(value ?? [])

  useEffect(() => {
    if (!enabled || json === current.current) return
    const now = Date.now()
    if (restoring.current) {
      restoring.current = false
    } else {
      if (now - lastAt.current > 700) {
        past.current = [...past.current.slice(-99), current.current]
      }
      future.current = []
      lastAt.current = now
    }
    current.current = json
    rerender((n) => n + 1)
  }, [enabled, json])

  const step = useCallback(
    (from: { current: string[] }, to: { current: string[] }) => {
      const target = from.current.at(-1)
      if (target === undefined) return
      from.current = from.current.slice(0, -1)
      to.current = [...to.current, current.current]
      restoring.current = true
      lastAt.current = 0
      onChange(JSON.parse(target))
    },
    [onChange],
  )
  return {
    canUndo: past.current.length > 0,
    canRedo: future.current.length > 0,
    undo: () => step(past, future),
    redo: () => step(future, past),
  }
}

// Dragging blocks anywhere in a builder's tree (MKSine's drag between columns). The outermost
// builder of a form shares the whole tree; nested builders (container slots) know their
// place from the block that owns them and the slot they are in.
interface BuilderRoot {
  tree: BuilderItem[]
  setTree: (tree: BuilderItem[]) => void
  dragging: BuilderItem | null
  setDragging: (item: BuilderItem | null) => void
}
const BuilderRootContext = createContext<BuilderRoot | null>(null)
const OwnerContext = createContext<BuilderItem | null>(null)
const SlotContext = createContext(-1)

/** A place between cards where a dragged block can be dropped. */
function DropZone({ onDrop, label }: { onDrop: () => void; label: string }) {
  const [over, setOver] = useState(false)
  return (
    <div
      role="presentation"
      aria-label={label}
      onDragOver={(e: DragEvent) => {
        e.preventDefault()
        e.dataTransfer.dropEffect = 'move'
        setOver(true)
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e: DragEvent) => {
        e.preventDefault()
        setOver(false)
        onDrop()
      }}
      className={'rounded-lg border-2 border-dashed transition-all ' + (over ? 'h-10 border-primary-500 bg-primary-50 dark:bg-primary-500/10' : 'h-3 border-primary-300/60 dark:border-primary-400/30')}
    />
  )
}

export function BuilderControl(props: Base & { field: Builder<any> }) {
  const parent = useContext(BuilderRootContext)
  const items = (Array.isArray(props.value) ? props.value : []) as BuilderItem[]
  const [dragging, setDragging] = useState<BuilderItem | null>(null)
  const root = useMemo<BuilderRoot>(() => ({ tree: items, setTree: (t) => props.onChange(t), dragging, setDragging }), [items, props.onChange, dragging])
  if (parent) return <BuilderList {...props} />
  return (
    <BuilderRootContext.Provider value={root}>
      <BuilderList {...props} />
    </BuilderRootContext.Provider>
  )
}

function BuilderList({ id, field, value, onChange, disabled, errors, ctx, name, idPrefix }: Base & { field: Builder<any> }) {
  const items = (Array.isArray(value) ? value : []) as BuilderItem[]
  const root = useContext(BuilderRootContext)
  const owner = useContext(OwnerContext)
  const slot = useContext(SlotContext)
  const place: Place = owner ? { owner, slot } : { owner: null, slot: -1 }
  const keys = useItemKeys(items.length)
  const [menuOpen, setMenuOpen] = useState(false)
  const [notice, setNotice] = useState('')
  const history = useHistory(field._history, value, onChange)
  const blockOf = (type: string): Block | undefined => field._blocks.find((b) => b.name === type)
  const canAdd = !disabled && (field._maxItems === undefined || items.length < field._maxItems)
  const dragging = root?.dragging ?? null
  // Drop zones: where the dragged block may go (a known type here, room, not inside itself).
  const canDrop =
    !!dragging && !disabled && !!blockOf(dragging.type) && !isWithin(dragging, owner) && (items.includes(dragging) || canAdd)
  const dropAt = (k: number) => {
    if (!root || !dragging) return
    const next = moveBlock(root.tree, dragging, place, k)
    if (next && next !== root.tree) root.setTree(next)
    root.setDragging(null)
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(JSON.stringify(items, null, 2))
      setNotice(t('builder.copied'))
    } catch {
      setNotice(t('builder.clipboardDenied'))
    }
  }

  async function paste() {
    let text: string | null = null
    try {
      text = await navigator.clipboard.readText()
    } catch {
      text = window.prompt(t('builder.pastePrompt'))
    }
    if (!text) return
    try {
      const parsed: unknown = JSON.parse(text)
      const list = (Array.isArray(parsed) ? parsed : [parsed]) as BuilderItem[]
      const valid = list.every((it) => it && typeof it === 'object' && typeof it.type === 'string' && blockOf(it.type) && (it.data === undefined || typeof it.data === 'object'))
      const room = field._maxItems === undefined ? Infinity : field._maxItems - items.length
      if (!valid || list.length === 0 || list.length > room) throw new Error('invalid')
      for (let i = 0; i < list.length; i++) keys.add()
      onChange([...items, ...list.map((it) => ({ type: it.type, data: it.data ?? {} }))])
      setNotice(t('builder.pasted', { count: list.length }))
    } catch {
      setNotice(t('builder.pasteInvalid'))
    }
  }

  const tool =
    'rounded-lg px-2.5 py-1 text-xs font-medium text-gray-600 ring-1 ring-gray-950/10 hover:bg-gray-50 disabled:opacity-40 dark:text-gray-300 dark:ring-white/10 dark:hover:bg-white/5'

  return (
    <div id={id} role="group" data-pb-builder="" className="space-y-3">
      {field._history && !disabled && (
        <div role="toolbar" aria-label={t('builder.tools')} className="flex flex-wrap items-center gap-1.5">
          <button type="button" className={tool} disabled={!history.canUndo} onClick={history.undo}>
            {t('builder.undo')}
          </button>
          <button type="button" className={tool} disabled={!history.canRedo} onClick={history.redo}>
            {t('builder.redo')}
          </button>
          <span className="mx-1 h-4 w-px bg-gray-200 dark:bg-white/10" aria-hidden="true" />
          <button type="button" className={tool} disabled={items.length === 0} onClick={() => void copy()}>
            {t('builder.copy')}
          </button>
          <button type="button" className={tool} disabled={!canAdd} onClick={() => void paste()}>
            {t('builder.paste')}
          </button>
          {notice && (
            <span role="status" className="ms-1 text-xs text-gray-500 dark:text-gray-400">
              {notice}
            </span>
          )}
        </div>
      )}
      {items.map((item, i) => {
        const block = blockOf(item.type)
        const data = item.data ?? {}
        return (
          <div key={keys.keys[i]} className="space-y-3">
          {canDrop && <DropZone label={t('builder.dropHere')} onDrop={() => dropAt(i)} />}
          <ItemCard
            marker={{ 'data-pb-card': String(i) }}
            drag={
              root && !disabled
                ? {
                    active: dragging === item,
                    start: () => root.setDragging(item),
                    end: () => root.setDragging(null),
                  }
                : undefined
            }
            title={block?._label ?? item.type}
            index={i}
            count={items.length}
            disabled={disabled}
            canRemove
            reorderable
            onMove={(j) => {
              keys.move(i, j)
              onChange(move(items, i, j))
            }}
            onRemove={() => {
              keys.remove(i)
              onChange(items.filter((_, j) => j !== i))
            }}
            onDuplicate={
              canAdd
                ? () => {
                    keys.add()
                    const copy = JSON.parse(JSON.stringify(item)) as BuilderItem
                    onChange([...items.slice(0, i + 1), copy, ...items.slice(i + 1)])
                  }
                : undefined
            }
          >
            {block ? (
              <OwnerContext.Provider value={item}>
              <Schema
                nodes={block._schema}
                columns={block._columns}
                ctx={itemContext(data, ctx)}
                values={data}
                errors={itemErrors(errors, `${name}.${i}.data.`)}
                idPrefix={`${idPrefix}${keys.keys[i]}-`}
                onChange={(child, v) => {
                  const r = applyChange(block._fields, data, child, v, ctx.operation, ctx.record)
                  onChange(items.map((it, j) => (j === i ? { ...it, data: r.values } : it)))
                }}
              />
              </OwnerContext.Provider>
            ) : (
              <p className="text-sm text-danger-600">{t('builder.unknown', { type: item.type })}</p>
            )}
            {errors[`${name}.${i}.type`]?.map((m) => (
              <p key={m} className="mt-2 text-sm text-danger-600">
                {m}
              </p>
            ))}
          </ItemCard>
          </div>
        )
      })}
      {canDrop && <DropZone label={t('builder.dropHere')} onDrop={() => dropAt(items.length)} />}
      {items.length === 0 && <p className="text-sm text-gray-500 dark:text-gray-400">{t('builder.empty')}</p>}
      {canAdd && (
        <div className="relative inline-block">
          <button
            type="button"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((o) => !o)}
            className="rounded-lg bg-white px-3 py-1.5 text-sm font-semibold text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 hover:ring-gray-950/20 active:scale-[0.98] dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10"
          >
            + {field._addLabel}
          </button>
          {menuOpen && (
            <div
              role="menu"
              onKeyDown={(e) => e.key === 'Escape' && setMenuOpen(false)}
              className="absolute z-20 mt-1 min-w-40 origin-top animate-pop rounded-xl bg-white p-1 shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10"
            >
              {field._blocks.map((block) => (
                <button
                  key={block.name}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    keys.add()
                    onChange([...items, { type: block.name, data: initialValues(block._fields) }])
                    setMenuOpen(false)
                  }}
                  className="block w-full rounded-md px-3 py-2 text-start text-sm hover:bg-gray-100 dark:hover:bg-white/10"
                >
                  {block._label ?? block.name}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function ItemCard({
  title,
  index,
  count,
  disabled,
  canRemove,
  reorderable,
  onMove,
  onRemove,
  onDuplicate,
  onIndent,
  onOutdent,
  marker,
  drag,
  children,
}: {
  title: string
  index: number
  count: number
  disabled?: boolean
  canRemove: boolean
  reorderable: boolean
  onMove: (to: number) => void
  onRemove: () => void
  onDuplicate?: () => void
  /** Tree repeaters: into the item before, and back out. */
  onIndent?: () => void
  onOutdent?: () => void
  /** data-pb-card / data-pb-slot: where the card sits, for focusBuilderBlock. */
  marker?: Record<string, string>
  /** Builder cards: drag the header to move the block anywhere in the builder. */
  drag?: { active: boolean; start: () => void; end: () => void }
  children: ReactNode
}) {
  const icon = 'rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-30 dark:hover:bg-white/10 [&_svg]:size-4'
  return (
    <div {...marker} className={'rounded-xl ring-1 ring-gray-950/10 transition-shadow dark:ring-white/10' + (drag?.active ? ' opacity-40' : '')}>
      <div
        className={'flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-2 dark:border-white/5' + (drag && !disabled ? ' cursor-grab active:cursor-grabbing' : '')}
        draggable={!!drag && !disabled}
        onDragStart={(e) => {
          if (!drag) return
          e.stopPropagation() // the innermost card is the one dragged
          e.dataTransfer.effectAllowed = 'move'
          e.dataTransfer.setData('text/plain', title)
          drag.start()
        }}
        onDragEnd={() => drag?.end()}
        title={drag && !disabled ? t('builder.dragHint') : undefined}
      >
        <span className="flex min-w-0 items-center gap-2 truncate text-sm font-medium">
          {drag && !disabled && (
            <span aria-hidden="true" className="text-gray-400">
              ⠿
            </span>
          )}
          {title}
        </span>
        {!disabled && (
          <div className="flex items-center gap-1">
            {reorderable && (
              <>
                <button type="button" className={icon} disabled={index === 0} onClick={() => onMove(index - 1)} aria-label={t('common.moveUp', { name: title })}>
                  <ChevronIcon dir="up" />
                </button>
                <button type="button" className={icon} disabled={index === count - 1} onClick={() => onMove(index + 1)} aria-label={t('common.moveDown', { name: title })}>
                  <ChevronIcon dir="down" />
                </button>
              </>
            )}
            {onOutdent && (
              <button type="button" className={icon} onClick={onOutdent} aria-label={t('repeater.outdent', { name: title })} title={t('repeater.outdent', { name: title })}>
                <ChevronIcon dir={direction() === 'rtl' ? 'right' : 'left'} />
              </button>
            )}
            {onIndent && (
              <button type="button" className={icon} onClick={onIndent} aria-label={t('repeater.indent', { name: title })} title={t('repeater.indent', { name: title })}>
                <ChevronIcon dir={direction() === 'rtl' ? 'left' : 'right'} />
              </button>
            )}
            {onDuplicate && (
              <button type="button" className={icon} onClick={onDuplicate} aria-label={t('builder.duplicate', { name: title })}>
                <CopyIcon />
              </button>
            )}
            {canRemove && (
              <button type="button" className={icon + ' hover:text-danger-600'} onClick={onRemove} aria-label={t('common.removeNamed', { name: title })}>
                <CloseIcon />
              </button>
            )}
          </div>
        )}
      </div>
      <div className="p-4">{children}</div>
    </div>
  )
}

/**
 * Opens a block of a builder field by its place ("2", or "2.0.1": block 1 of slot 0 of
 * block 2), e.g. after a click on it in a preview: scrolls its card into view, marks it
 * for a moment and focuses its first field. Returns whether the block was found.
 */
export function focusBuilderBlock(builderId: string, path: string): boolean {
  let builder: Element | null = document.getElementById(builderId)
  const parts = path.split('.').filter((p) => p !== '')
  let card: HTMLElement | null = null
  for (let k = 0; builder && k < parts.length; k += 2) {
    const list: Element = builder
    card = [...list.querySelectorAll<HTMLElement>(`[data-pb-card="${CSS.escape(parts[k])}"]`)].find((el) => el.closest('[data-pb-builder]') === list) ?? null
    if (!card || k + 1 >= parts.length) break
    // The slot (a repeater item of this card), then the builder inside it.
    const slot = [...card.querySelectorAll<HTMLElement>(`[data-pb-slot="${CSS.escape(parts[k + 1])}"]`)].find((el) => el.parentElement?.closest('[data-pb-card]') === card)
    builder = slot ? [...slot.querySelectorAll('[data-pb-builder]')].find((el) => el.closest('[data-pb-slot]') === slot) ?? null : null
    if (!builder) break
  }
  if (!card) return false
  card.style.scrollMarginTop = '5rem' // below the sticky top bar
  card.scrollIntoView({ block: 'start', behavior: 'smooth' })
  card.classList.add('ring-2', 'ring-primary-500')
  window.setTimeout(() => card?.classList.remove('ring-2', 'ring-primary-500'), 1600)
  const own = [...card.querySelectorAll<HTMLElement>('input:not([type=hidden]):not([type=file]), textarea, select, [contenteditable=true]')].find(
    (el) => el.closest('[data-pb-card]') === card,
  )
  own?.focus({ preventScroll: true })
  return true
}
