import { toast } from 'vue-sonner'
import type { ToastType } from '@/types'

export function useToast() {
  const show = (message: string, type: ToastType = 'info') => {
    if (type === 'success') toast.success(message)
    else if (type === 'error') toast.error(message)
    else toast(message)
  }

  return {
    show,
    success: (message: string) => toast.success(message),
    error: (message: string) => toast.error(message),
    info: (message: string) => toast(message),
  }
}
