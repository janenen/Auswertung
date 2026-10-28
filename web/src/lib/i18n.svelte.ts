// UI translations. German texts are the keys; en.ts defines the key set
// and gu.ts/ml.ts are type-checked against it.
import { en, type Key } from './locales/en'
import { gu } from './locales/gu'
import { ml } from './locales/ml'

export const locales = { de: 'Deutsch', en: 'English', gu: 'ગુજરાતી', ml: 'മലയാളം' } as const
export type Locale = keyof typeof locales

const dictionaries: Record<Exclude<Locale, 'de'>, Record<Key, string>> = { en, gu, ml }
const STORAGE_KEY = 'auswertung.locale'

function initial(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && saved in locales) return saved as Locale
  } catch {
    // storage unavailable: fall back to the browser language
  }
  const browser = navigator.language.slice(0, 2)
  return browser in locales ? (browser as Locale) : 'de'
}

export const i18n = $state({ locale: initial() })

$effect.root(() => {
  $effect(() => {
    document.documentElement.lang = i18n.locale
    try {
      localStorage.setItem(STORAGE_KEY, i18n.locale)
    } catch {
      // not persisted
    }
  })
})

function format(text: string, params?: Record<string, string | number>) {
  return params ? text.replace(/\{(\w+)\}/g, (m, k) => (k in params ? String(params[k]) : m)) : text
}

function lookup(text: string): string | undefined {
  return i18n.locale === 'de' ? text : (dictionaries[i18n.locale] as Record<string, string>)[text]
}

/** Translates a UI text; params replace {name} placeholders. */
export function t(key: Key, params?: Record<string, string | number>): string {
  return format(lookup(key) ?? key, params)
}

// Server messages with values, matched by their German pattern.
const patterns: Key[] = ['Falscher Scheibentyp: {a} statt {b}', 'Speichern fehlgeschlagen: {a}', 'Rika SAG-2 an {a}']
const matchers = patterns.map((p) => {
  const names: string[] = []
  const source = p.replace(/[.*+?^$()|[\]\\]/g, '\\$&').replace(/\{(\w+)\}/g, (_, n) => (names.push(n), '(.+)'))
  return { key: p, re: new RegExp(`^${source}$`), names }
})

/** Translates a text from the server (scoring mode, machine, message); unknown texts stay German. */
export function tx(text: string): string {
  const direct = lookup(text)
  if (direct) return direct
  for (const m of matchers) {
    const found = m.re.exec(text)
    if (found) return t(m.key, Object.fromEntries(m.names.map((n, i) => [n, tx(found[i + 1])])))
  }
  return text
}
