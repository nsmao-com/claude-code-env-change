import { ref } from 'vue'

const isOpen = ref(false)
const title = ref('')
const message = ref('')
const confirmType = ref<'danger' | 'warning' | 'info'>('info')
let resolvePromise: ((value: boolean) => void) | null = null
let settled = false

function finish(value: boolean) {
  if (settled) return
  settled = true
  isOpen.value = false
  const resolve = resolvePromise
  resolvePromise = null
  resolve?.(value)
}

export function useConfirm() {
  const show = (
    dialogTitle: string,
    dialogMessage: string,
    type: 'danger' | 'warning' | 'info' = 'info'
  ): Promise<boolean> => {
    title.value = dialogTitle
    message.value = dialogMessage
    confirmType.value = type
    settled = false
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  const confirm = () => finish(true)
  const cancel = () => finish(false)

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
