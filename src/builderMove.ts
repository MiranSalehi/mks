/**
 * Moving a block anywhere in a builder's tree (MKSine's drag between columns). Blocks are
 * {type, data}; a container's data.children is a list of slots, each {blocks: [...]}.
 * A place is a list of blocks: the top level (owner null), or a slot of a container block.
 */
export interface TreeBlock {
  type: string
  data?: Record<string, unknown>
}

export interface Place {
  owner: TreeBlock | null
  slot: number
}

function slotsOf(b: TreeBlock): { blocks?: unknown }[] | null {
  const children = b.data?.children
  return Array.isArray(children) ? (children as { blocks?: unknown }[]) : null
}

function blocksOf(slot: { blocks?: unknown } | undefined): TreeBlock[] {
  return Array.isArray(slot?.blocks) ? (slot.blocks as TreeBlock[]) : []
}

/** Whether target is block itself or anywhere inside it. */
export function isWithin(block: TreeBlock, target: TreeBlock | null): boolean {
  if (!target) return false
  if (block === target) return true
  return (slotsOf(block) ?? []).some((s) => blocksOf(s).some((b) => isWithin(b, target)))
}

/** Where a block is: its list and index, or null. */
export function findBlock(tree: TreeBlock[], item: TreeBlock, place: Place = { owner: null, slot: -1 }): { place: Place; index: number } | null {
  const i = tree.indexOf(item)
  if (i >= 0) return { place, index: i }
  for (const b of tree) {
    const slots = slotsOf(b)
    if (!slots) continue
    for (let s = 0; s < slots.length; s++) {
      const found = findBlock(blocksOf(slots[s]), item, { owner: b, slot: s })
      if (found) return found
    }
  }
  return null
}

const samePlace = (a: Place, b: Place) => a.owner === b.owner && (a.owner === null || a.slot === b.slot)

/**
 * The tree with item moved to target at index (an index of the target list as it is now,
 * before the move). Null when the move makes no sense: the item is not in the tree, or the
 * target is the item itself or inside it.
 */
export function moveBlock<T extends TreeBlock>(tree: T[], item: T, target: Place, index: number): T[] | null {
  const from = findBlock(tree, item)
  if (!from || isWithin(item, target.owner)) return null
  if (target.owner && (!findBlock(tree, target.owner) || target.slot < 0 || target.slot >= (slotsOf(target.owner)?.length ?? 0))) return null
  let at = index
  if (samePlace(from.place, target) && from.index < index) at--
  if (samePlace(from.place, target) && from.index === at) return tree

  const walk = (list: TreeBlock[], place: Place): TreeBlock[] => {
    const out: TreeBlock[] = list
      .filter((b) => b !== item)
      .map((b) => {
        const slots = slotsOf(b)
        if (!slots) return b
        const nextSlots = slots.map((slot, s) => ({ ...slot, blocks: walk(blocksOf(slot), { owner: b, slot: s }) }))
        return { ...b, data: { ...b.data, children: nextSlots } }
      })
    if (samePlace(place, target)) out.splice(Math.max(0, Math.min(at, out.length)), 0, item)
    return out
  }
  return walk(tree, { owner: null, slot: -1 }) as T[]
}
