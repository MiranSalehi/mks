import { useSyncExternalStore } from 'react'

/**
 * The panel's look (MKSine's admin appearance): a preset, the primary color, the color
 * mode, the sidebar style, corner radius, font size and density. The appearance plugin
 * sends the company look and the user's own in meta.extra.appearance; setAppearance
 * applies one (as data attributes on <html>, which styles.css turns into variables).
 */
export interface Appearance {
  preset?: Preset
  primary?: Primary
  mode?: 'light' | 'dark' | 'system'
  rail?: 'light' | 'dark' | 'brand'
  radius?: 'sharp' | 'medium' | 'round'
  font?: 'small' | 'normal' | 'large'
  density?: 'compact' | 'comfortable'
  /** Form fields: outlined (a frame), filled (a tinted box), underline, or floating labels. */
  field?: 'outlined' | 'filled' | 'underline' | 'floating'
  /** Form labels above fields, or beside them on wide screens. */
  label?: 'top' | 'side'
}

export type Preset = 'modern' | 'classic' | 'minimal' | 'soft' | 'bold'
export type Primary = 'iris' | 'indigo' | 'blue' | 'cyan' | 'emerald' | 'violet' | 'orange' | 'fuchsia' | 'slate'

export const PRIMARIES: Primary[] = ['iris', 'indigo', 'blue', 'cyan', 'emerald', 'violet', 'orange', 'fuchsia', 'slate']

/** What each preset sets; explicit choices win over it. */
export const PRESETS: Record<Preset, Required<Omit<Appearance, 'preset' | 'mode'>>> = {
  modern: { primary: 'iris', rail: 'light', radius: 'medium', font: 'normal', density: 'comfortable', field: 'outlined', label: 'top' },
  classic: { primary: 'blue', rail: 'dark', radius: 'sharp', font: 'normal', density: 'comfortable', field: 'outlined', label: 'side' },
  minimal: { primary: 'slate', rail: 'light', radius: 'sharp', font: 'small', density: 'compact', field: 'underline', label: 'top' },
  soft: { primary: 'violet', rail: 'light', radius: 'round', font: 'normal', density: 'comfortable', field: 'filled', label: 'top' },
  bold: { primary: 'orange', rail: 'brand', radius: 'round', font: 'large', density: 'comfortable', field: 'floating', label: 'top' },
}

export type ResolvedAppearance = Required<Omit<Appearance, 'mode'>> & { mode?: Appearance['mode'] }

/** The look with the preset's choices filled in. */
export function resolveAppearance(a: Appearance | null | undefined): ResolvedAppearance {
  const preset: Preset = a?.preset && a.preset in PRESETS ? a.preset : 'modern'
  const base = PRESETS[preset]
  const pick = <K extends keyof typeof base>(k: K, allowed: readonly string[]) =>
    (a?.[k] && allowed.includes(a[k] as string) ? a[k] : base[k]) as (typeof base)[K]
  return {
    preset,
    primary: pick('primary', PRIMARIES),
    rail: pick('rail', ['light', 'dark', 'brand']),
    radius: pick('radius', ['sharp', 'medium', 'round']),
    font: pick('font', ['small', 'normal', 'large']),
    density: pick('density', ['compact', 'comfortable']),
    field: pick('field', ['outlined', 'filled', 'underline', 'floating']),
    label: pick('label', ['top', 'side']),
    mode: a?.mode && ['light', 'dark', 'system'].includes(a.mode) ? a.mode : undefined,
  }
}

let current: ResolvedAppearance = resolveAppearance(null)
const listeners = new Set<() => void>()

/** Applies a look (also for a live preview); null goes back to the default. */
export function setAppearance(a: Appearance | null | undefined) {
  const next = resolveAppearance(a)
  if (JSON.stringify(next) === JSON.stringify(current)) return
  current = next
  if (typeof document !== 'undefined') {
    const d = document.documentElement.dataset
    d.primary = next.primary
    d.radius = next.radius
    d.font = next.font
    d.density = next.density
    d.field = next.field
    d.label = next.label
  }
  listeners.forEach((l) => l())
}

/** The look in effect, for components that draw it (the sidebar). */
export function useAppearance(): ResolvedAppearance {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l)
      return () => listeners.delete(l)
    },
    () => current,
    () => current,
  )
}
