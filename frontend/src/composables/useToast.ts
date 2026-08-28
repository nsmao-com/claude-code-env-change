import { ref } from 'vue'
import type { Toast, ToastType } from '@/types'

const toasts = ref<Toast[]>([])
const timers = new Map<number, ReturnType<typeof setTimeout>>()
let seq = 1
const MAX_VISIBLE = 3

function durationFor(type: ToastType) {
  return type === 'error' ? 3800 : 2400
}

function dismiss(id: number) {
  const timer = timers.get(id)
  if (timer) {
    clearTimeout(timer)
    timers.delete(id)
  }
  toasts.value = toasts.value.filter(item => item.id !== id)
}

function arm(id: number, type: ToastType) {
  const prev = timers.get(id)
  if (prev) clearTimeout(prev)
  timers.set(id, setTimeout(() => dismiss(id), durationFor(type)))
}

function show(message: string, type: ToastType = 'info') {
  const text = String(message || '').trim()
  if (!text) return

  const existing = toasts.value.find(item => item.message === text && item.type === type)
  if (existing) {
    arm(existing.id, type)
    toasts.value = [...toasts.value.filter(item => item.id !== existing.id), existing]
    return
  }

  const id = seq++
  toasts.value = [...toasts.value, { id, message: text, type }]
  while (toasts.value.length > MAX_VISIBLE) {
    dismiss(toasts.value[0].id)
  }
  arm(id, type)
}

export function useToast() {
  return {
    toasts,
    show,
    dismiss,
    success: (message: string) => show(message, 'success'),
    error: (message: string) => show(message, 'error'),
    info: (message: string) => show(message, 'info'),
  }
}
