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
    // 前一个弹窗还没被答复时又来了新的：先把旧的按"取消"结算，
    // 否则旧调用方的 await 永远挂起，后续逻辑静默死亡
    finish(false)
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
