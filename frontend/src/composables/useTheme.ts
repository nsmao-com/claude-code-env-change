import { ref, watch } from 'vue'

const isDark = ref(false)
let initialized = false

// 立即初始化主题（不等待 mounted）
function initTheme() {
  if (initialized) return
  initialized = true

  const stored = localStorage.getItem('theme')
  if (stored) {
    isDark.value = stored === 'dark'
  } else {
    // 默认使用亮色模式
    isDark.value = false
  }

  // 立即应用
  applyTheme(isDark.value)
}

function applyTheme(dark: boolean) {
  if (dark) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
  localStorage.setItem('theme', dark ? 'dark' : 'light')
}

// 初始化
initTheme()

export function useTheme() {
  const toggle = () => {
    isDark.value = !isDark.value
  }

  const setDark = (value: boolean) => {
    isDark.value = value
  }

  // 监听变化并更新 DOM 和 localStorage
  watch(isDark, (value) => {
    applyTheme(value)
  })

  return {
    isDark,
    toggle,
    setDark
  }
}
