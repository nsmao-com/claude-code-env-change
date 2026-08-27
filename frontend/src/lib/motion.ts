export const easeOut = [0.22, 1, 0.36, 1] as const

export const motionDuration = {
  press: 0.14,
  ui: 0.22,
  overlay: 0.28,
  page: 0.24,
} as const

export function listEnter(index: number) {
  return {
    initial: { opacity: 0, y: 14 },
    animate: { opacity: 1, y: 0 },
    transition: {
      duration: motionDuration.ui,
      ease: easeOut,
      delay: Math.min(index, 8) * 0.04,
    },
  }
}

export const fadeEnter = {
  initial: { opacity: 0, y: 12 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: motionDuration.overlay, ease: easeOut },
}

export const pageEnter = {
  initial: { opacity: 0, y: 16 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -10 },
  transition: { duration: motionDuration.page, ease: easeOut },
}

export const pressSpring = { scale: 0.985 }
export const hoverLift = { y: -2 }
