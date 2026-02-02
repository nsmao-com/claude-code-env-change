import { ref, computed } from 'vue'
import type { Toast, ToastType } from '@/types'

const toasts = ref<Toast[]>([])
let toastId = 0

export function useToast() {
  const show = (message: string, type: ToastType = 'info') => {
    const id = ++toastId
    toasts.value.push({ id, message, type })

    setTimeout(() => {
      remove(id)
    }, 3000)
  }

  const remove = (id: number) => {
    const index = toasts.value.findIndex(t => t.id === id)
    if (index > -1) {
      toasts.value.splice(index, 1)
    }
  }

  const success = (message: string) => show(message, 'success')
  const error = (message: string) => show(message, 'error')
  const info = (message: string) => show(message, 'info')

  return {
    toasts: computed(() => toasts.value),
    show,
    remove,
    success,
    error,
    info
  }
}
