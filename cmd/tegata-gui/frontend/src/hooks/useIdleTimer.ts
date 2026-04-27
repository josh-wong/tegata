import { useCallback, useEffect, useRef } from "react"
import { App as WailsApp } from "@/lib/wails"

const ACTIVITY_EVENTS = ["mousedown", "keydown", "touchstart", "scroll"] as const

// How often (ms) we sync the backend idle timer during active use.
const BACKEND_RESET_INTERVAL = 30_000

export function useIdleTimer(timeoutMs: number, onIdle: () => void) {
  const lastActivity = useRef(0)
  const lastBackendReset = useRef(0)

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
      const now = Date.now()
      if (now - lastActivity.current >= timeoutMs) {
        // Reset the baseline so the timer cannot re-fire before the caller
        // disables it (e.g. while an async lock transition is in flight).
        lastActivity.current = now
        onIdle()
      } else if (
        lastActivity.current > lastBackendReset.current &&
        now - lastBackendReset.current >= BACKEND_RESET_INTERVAL
      ) {
        // Sync the backend idle timer only when there has been new activity
        // since the last sync. This prevents resetting the backend timer during
        // a period of true inactivity (e.g., user idle for 4 of 5 minutes).
        lastBackendReset.current = now
        WailsApp.ResetIdle().catch(() => {})
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
