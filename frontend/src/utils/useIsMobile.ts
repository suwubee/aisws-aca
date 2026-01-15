import { onMounted, onUnmounted, ref } from 'vue'

function computeIsMobile(): boolean {
  if (typeof window === 'undefined') return false

  const isCoarsePointer =
    typeof window.matchMedia === 'function' && window.matchMedia('(pointer: coarse)').matches

  return window.innerWidth <= 768 || (isCoarsePointer && window.innerWidth <= 1024)
}

export function useIsMobile() {
  const isMobile = ref(computeIsMobile())

  function update() {
    isMobile.value = computeIsMobile()
  }

  onMounted(() => {
    update()
    window.addEventListener('resize', update)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', update)
  })

  return { isMobile, update }
}

