export const PLATFORM_ITEMS = [
  { key: 'claude-code', brand: 'claude', label: 'Claude', onClass: 'border-green-500/30 bg-green-500/15 text-green-600 dark:text-green-400', offClass: 'border-border bg-muted/40 text-muted-foreground' },
  { key: 'codex', brand: 'codex', label: 'Codex', onClass: 'border-brand/30 bg-brand/10 text-brand', offClass: 'border-border bg-muted/40 text-muted-foreground' },
  { key: 'antigravity', brand: 'antigravity', label: 'Antigravity', onClass: 'border-blue-500/30 bg-blue-500/15 text-blue-600 dark:text-blue-400', offClass: 'border-border bg-muted/40 text-muted-foreground' },
  { key: 'opencode', brand: 'opencode', label: 'OpenCode', onClass: 'border-foreground/20 bg-foreground/8 text-foreground', offClass: 'border-border bg-muted/40 text-muted-foreground' },
  { key: 'grok', brand: 'grok', label: 'Grok', onClass: 'border-stone-500/30 bg-stone-500/15 text-stone-600 dark:text-stone-400', offClass: 'border-border bg-muted/40 text-muted-foreground' },
] as const

export type PlatformKey = (typeof PLATFORM_ITEMS)[number]['key']

export function togglePlatformList(list: string[] | undefined, key: string): string[] {
  const current = list || []
  return current.includes(key) ? current.filter(item => item !== key) : [...current, key]
}
