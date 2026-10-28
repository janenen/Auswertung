import type { Meta } from './api'

// Static data (targets, modes) loaded once at startup by App.svelte.
export const meta = $state<Meta>({
  targets: {},
  targetTypes: [],
  modi: [],
  leagueModus: '',
  leagues: [],
  simulator: false,
})

export function today(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`
}

export const fmt1 = (v: number) => v.toFixed(1)
export const fmt2 = (v: number) => v.toFixed(2)

/** Result of a list of shots in the scoring of the competition. */
export const result = (s: { summe: number; summeGanz: number }, decimal: boolean) =>
  decimal ? fmt1(s.summe) : String(s.summeGanz)

export const ringValue = (ringe: number, decimal: boolean) =>
  decimal ? fmt1(ringe) : String(Math.floor(ringe))
