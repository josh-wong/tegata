import { useCallback, useEffect, useRef } from "react"

const ACTIVITY_EVENTS = ["mousedown", "keydown", "touchstart", "scroll"] as const

export function useIdleTimer(timeoutMs: number, onIdle: () => void) {
  const lastActivity = useRef(Date.now())

  const resetTimer = useCallback(() => {
    lastActivity.current = Date.now()
  }, [])

  useEffect(() => {
    const handler = () => {
      lastActivity.current = Date.now()
    }

    for (const event of ACTIVITY_EVENTS) {
      document.addEventListener(event, handler, { passive: true })
    }

    const interval = setInterval(() => {
      if (Date.now() - lastActivity.current >= timeoutMs) {
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
