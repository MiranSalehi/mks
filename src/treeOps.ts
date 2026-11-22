/**
 * Moves within a tree of records (a repeater with tree(): each item has a list of items
 * under `key`, e.g. menu entries with children). A path is the indexes from the top list:
 * [2, 0] is the first child of the third item. All functions return a new tree.
 */
type Node = Record<string, unknown>

function childrenOf(node: Node, key: string): Node[] {
  return Array.isArray(node[key]) ? (node[key] as Node[]) : []
}

/** The list at a parent path ([] is the top list). */
export function listAt(tree: Node[], parent: number[], key: string): Node[] | null {
  let list = tree
  for (const i of parent) {
    const node = list[i]
    if (!node) return null
    list = childrenOf(node, key)
  }
  return list
}

/** The tree with the list at a parent path replaced. */
function withList(tree: Node[], parent: number[], key: string, list: Node[]): Node[] {
  if (parent.length === 0) return list
  const [i, ...rest] = parent
  return tree.map((node, j) => (j === i ? { ...node, [key]: withList(childrenOf(node, key), rest, key, list) } : node))
}

/** How many levels a node spans: 1 without children. */
export function treeHeight(node: Node, key: string): number {
  return 1 + Math.max(0, ...childrenOf(node, key).map((c) => treeHeight(c, key)))
}

/**
 * Makes the item at path the last child of the item before it. Null when there is no item
 * before it, or when the item and its children would go deeper than maxDepth levels.
 */
export function indentItem(tree: Node[], path: number[], key: string, maxDepth: number): Node[] | null {
  const parent = path.slice(0, -1)
  const i = path[path.length - 1]
  const list = listAt(tree, parent, key)
  if (!list || i <= 0 || i >= list.length) return null
  const node = list[i]
  if (parent.length + 1 + treeHeight(node, key) > maxDepth) return null
  const sibling = list[i - 1]
  const next = list.filter((_, j) => j !== i).map((n, j) => (j === i - 1 ? { ...sibling, [key]: [...childrenOf(sibling, key), node] } : n))
  return withList(tree, parent, key, next)
}

/** Moves the item at path out of its parent, to just after the parent. Null at the top. */
export function outdentItem(tree: Node[], path: number[], key: string): Node[] | null {
  if (path.length < 2) return null
  const parent = path.slice(0, -1)
  const i = path[path.length - 1]
  const list = listAt(tree, parent, key)
  if (!list || i < 0 || i >= list.length) return null
  const node = list[i]
  const removed = withList(tree, parent, key, list.filter((_, j) => j !== i))
  const grand = parent.slice(0, -1)
  const at = parent[parent.length - 1] + 1
  const outer = listAt(removed, grand, key)
  if (!outer) return null
  return withList(removed, grand, key, [...outer.slice(0, at), node, ...outer.slice(at)])
}
