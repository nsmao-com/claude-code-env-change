function readAppMethod(name: string): ((...args: unknown[]) => Promise<unknown>) | undefined {
  const fn = (window as unknown as {
    go?: { main?: { App?: Record<string, unknown> } }
  }).go?.main?.App?.[name]
  return typeof fn === 'function' ? fn as (...args: unknown[]) => Promise<unknown> : undefined
}

export function unwrap(raw: unknown): unknown {
  if (raw == null) return raw
  if (typeof raw === 'string') {
    const text = raw.trim()
    if (text.startsWith('{') || text.startsWith('[')) {
      try {
        return unwrap(JSON.parse(text))
      } catch {
        return raw
      }
    }
  }
  return raw
}

export function asRecord(raw: unknown): Record<string, unknown> | null {
  const value = unwrap(raw)
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

export function pickBool(raw: unknown, ...keys: string[]): boolean {
  const rec = asRecord(raw)
  if (!rec) return false
  for (const key of keys) {
    const value = rec[key]
    if (value === true || value === 1 || value === 'true') return true
  }
  return false
}

export function pickText(raw: unknown, ...keys: string[]): string {
  const rec = asRecord(raw)
  if (!rec) return typeof unwrap(raw) === 'string' ? String(unwrap(raw)) : ''
  for (const key of keys) {
    if (typeof rec[key] === 'string' && rec[key]) return rec[key] as string
  }
  return ''
}

export function pickNum(raw: unknown, ...keys: string[]): number {
  const rec = asRecord(raw)
  if (!rec) return 0
  for (const key of keys) {
    if (typeof rec[key] === 'number' && Number.isFinite(rec[key])) return rec[key] as number
  }
  return 0
}

export function onAppEvent(name: string, handler: (data: unknown) => void): () => void {
  const runtime = (window as unknown as {
    runtime?: { EventsOn?: (event: string, cb: (...args: unknown[]) => void) => () => void }
  }).runtime
  if (!runtime?.EventsOn) return () => {}
  return runtime.EventsOn(name, (...args: unknown[]) => {
    handler(args.length === 1 ? args[0] : args)
  })
}

export async function callApp<T>(name: string, ...args: unknown[]): Promise<T> {
  for (let i = 0; i < 25; i++) {
    const fn = readAppMethod(name)
    if (fn) return fn(...args) as Promise<T>
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  throw new Error(`后端尚未加载 ${name}，请关掉窗口重新打开软件`)
}
