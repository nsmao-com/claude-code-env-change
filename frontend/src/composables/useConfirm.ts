import { ref } from 'vue'

// 确认弹窗状态
const isOpen = ref(false)
const title = ref('')
const message = ref('')
const confirmType = ref<'danger' | 'warning' | 'info'>('info')
let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirm() {
  const show = (
    dialogTitle: string,
    dialogMessage: string,
    type: 'danger' | 'warning' | 'info' = 'info'
  ): Promise<boolean> => {
    title.value = dialogTitle
    message.value = dialogMessage
    confirmType.value = type
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  const confirm = () => {
    isOpen.value = false
    resolvePromise?.(true)
    resolvePromise = null
  }

  const cancel = () => {
    isOpen.value = false
    resolvePromise?.(false)
    resolvePromise = null
  }

  return {
    isOpen,
    title,
    message,
    confirmType,
    show,
    confirm,
    cancel
  }
}
