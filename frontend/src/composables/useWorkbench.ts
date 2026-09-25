import { ref } from 'vue'
import { useI18n } from './useI18n'
import { useToast } from './useToast'
import { callService } from '@/services/appBridge'

export const workbench = <T>(name: string, ...args: unknown[]) =>
  callService<T>('WorkbenchService', name, ...args)
export function useWorkbench() {
  const { locale } = useI18n()
  const tx = (zh: string, en: string) => (locale.value === 'zh' ? zh : en)
  const busy = ref(false)
  const error = ref('')
  const toast = useToast()
  async function run<T>(action: () => Promise<T>, message?: string): Promise<T | undefined> {
    if (busy.value) return
    busy.value = true
    error.value = ''
    try {
      const value = await action()
      if (message) toast.success(message)
      return value
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      busy.value = false
    }
  }
  return { tx, busy, error, run }
}
