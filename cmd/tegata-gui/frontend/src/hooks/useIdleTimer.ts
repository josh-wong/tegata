import { useCallback, useEffect, useRef } from "react"

const ACTIVITY_EVENTS = ["mousedown", "keydown", "touchstart", "scroll"] as const

export function useIdleTimer(timeoutMs: number, onIdle: () => void) {
  const lastActivity = useRef(0)

  const resetTimer = useCallback(() => {
    lastActivity.current = Date.now()
  }, [])

  useEffect(() => {
    if (timeoutMs <= 0) return

    lastActivity.current = Date.now()

    const handler = () => {
      lastActivity.current = Date.now()
    }

    for (const event of ACTIVITY_EVENTS) {
      document.addEventListener(event, handler, { passive: true })
    }

    const interval = setInterval(() => {
      if (Date.now() - lastActivity.current >= timeoutMs) {
        // Reset the baseline so the timer cannot re-fire before the caller
        // disables it (e.g. while an async lock transition is in flight).
        lastActivity.current = Date.now()
        onIdle()
      }
    }, 5000)

    return () => {
      for (const event of ACTIVITY_EVENTS) {
        document.removeEventListener(event, handler)
      }
      clearInterval(interval)
    }
  }, [timeoutMs, onIdle])

  return { resetTimer } as const
}
