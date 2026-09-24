import { zh } from './locales/zh'
import { en } from './locales/en'

import type { DeepString } from './types'

export type Locale = 'zh' | 'en'
export type MessageTree = DeepString<typeof zh>

export const messages: Record<Locale, MessageTree> = {
  zh,
  en,
}

export function isLocale(value: unknown): value is Locale {
  return value === 'zh' || value === 'en'
}

export function lookupMessage(tree: unknown, path: string): string | undefined {
  const parts = path.split('.')
  let cur: unknown = tree
  for (const part of parts) {
    if (!cur || typeof cur !== 'object' || !(part in cur)) return undefined
    cur = (cur as Record<string, unknown>)[part]
  }
  return typeof cur === 'string' ? cur : undefined
}

export function formatMessage(template: string, vars?: Record<string, string | number>) {
  if (!vars) return template
  return template.replace(/\{(\w+)\}/g, (_, key: string) => String(vars[key] ?? `{${key}}`))
}
