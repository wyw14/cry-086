import { onBeforeUnmount, onMounted } from 'vue'

export function usePolling(action: () => Promise<unknown>, interval = 5000) {
  let timer: number | undefined
  const execute = async () => {
    await action()
    timer = window.setTimeout(execute, interval)
  }
  onMounted(execute)
  onBeforeUnmount(() => { if (timer !== undefined) window.clearTimeout(timer) })
}
