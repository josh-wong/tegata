import { describe, expect, it, beforeEach, afterEach, vi } from "vitest"
import { renderHook, act } from "@testing-library/react"
import { App as WailsApp } from "@/lib/wails"
import { useIdleTimer } from "./useIdleTimer"

describe("useIdleTimer", () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it("fires onIdle callback after timeoutMs of inactivity", () => {
    const onIdle = vi.fn()
    renderHook(() => useIdleTimer(10000, onIdle))

    // The hook polls every 5000ms. Advance past the timeout.
    act(() => {
      vi.advanceTimersByTime(15000)
    })

    expect(onIdle).toHaveBeenCalled()
  })

  it("does NOT fire if activity event occurs before timeout", () => {
    const onIdle = vi.fn()
    renderHook(() => useIdleTimer(10000, onIdle))

    // Advance 7s (within timeout), then trigger activity
    act(() => {
      vi.advanceTimersByTime(7000)
    })

    act(() => {
      document.dispatchEvent(new Event("mousedown"))
    })

    // Advance another 7s (should be under timeout from last activity)
    act(() => {
      vi.advanceTimersByTime(7000)
    })

    expect(onIdle).not.toHaveBeenCalled()
  })

  it("cleans up event listeners and interval on unmount", () => {
    const onIdle = vi.fn()
    const { unmount } = renderHook(() => useIdleTimer(10000, onIdle))

    unmount()

    // Advance well past timeout — callback should not fire
    act(() => {
      vi.advanceTimersByTime(30000)
    })

    expect(onIdle).not.toHaveBeenCalled()
  })

  it("does nothing when timeoutMs is 0", () => {
    const onIdle = vi.fn()
    renderHook(() => useIdleTimer(0, onIdle))

    act(() => {
      vi.advanceTimersByTime(30000)
    })

    expect(onIdle).not.toHaveBeenCalled()
  })

  it("does nothing when timeoutMs is negative", () => {
    const onIdle = vi.fn()
    renderHook(() => useIdleTimer(-1, onIdle))

    act(() => {
      vi.advanceTimersByTime(30000)
    })

    expect(onIdle).not.toHaveBeenCalled()
  })

  it("syncs backend idle timer after activity when BACKEND_RESET_INTERVAL elapses", () => {
    const onIdle = vi.fn()
    const resetIdleMock = vi.mocked(WailsApp.ResetIdle)
    resetIdleMock.mockClear()

    renderHook(() => useIdleTimer(10 * 60 * 1000, onIdle)) // 10-minute timeout

    // Trigger user activity.
    act(() => {
      document.dispatchEvent(new Event("mousedown"))
    })

    // Advance 35s — past the 30s BACKEND_RESET_INTERVAL. ResetIdle should fire.
    act(() => {
      vi.advanceTimersByTime(35_000)
    })

    expect(resetIdleMock).toHaveBeenCalled()
    expect(onIdle).not.toHaveBeenCalled()
  })

  it("does NOT sync backend timer when no activity occurred since last sync", () => {
    const onIdle = vi.fn()
    const resetIdleMock = vi.mocked(WailsApp.ResetIdle)
    resetIdleMock.mockClear()

    renderHook(() => useIdleTimer(10 * 60 * 1000, onIdle))

    // Advance 5s to trigger the first poll, which initialises lastBackendReset
    // (lastActivity starts at Date.now() so lastActivity > lastBackendReset initially).
    act(() => {
      vi.advanceTimersByTime(5_000)
    })
    resetIdleMock.mockClear()

    // No further activity. Advance another 35s — no new activity, so ResetIdle
    // must NOT be called again.
    act(() => {
      vi.advanceTimersByTime(35_000)
    })

    expect(resetIdleMock).not.toHaveBeenCalled()
  })

  it("resets activity baseline when re-enabled after being disabled (login transition)", () => {
    // Simulate the pattern used in App.tsx: pass 0 while pre-login, then the
    // real timeout after the user authenticates. The timer must not fire
    // immediately after enabling — the activity baseline should reset so that
    // time spent on pre-login screens does not count toward the idle window.
    const onIdle = vi.fn()
    let timeout = 0
    const { rerender } = renderHook(() => useIdleTimer(timeout, onIdle))

    // Advance 9 minutes while the timer is disabled (timeout = 0).
    act(() => {
      vi.advanceTimersByTime(9 * 60 * 1000)
    })
    expect(onIdle).not.toHaveBeenCalled()

    // Simulate login: enable the timer with a 5-minute window.
    timeout = 5 * 60 * 1000
    rerender()

    // Advance 4 minutes — the 9 pre-login minutes must not count; the user
    // should still have 1 minute of idle budget remaining.
    act(() => {
      vi.advanceTimersByTime(4 * 60 * 1000)
    })
    expect(onIdle).not.toHaveBeenCalled()

    // Advance 1 minute and 5 seconds — past the 5-minute window from login.
    // The hook polls every 5 s, so the callback fires at least once in this
    // window. The baseline is reset before onIdle fires, so the callback fires
    // at most once per idle window even if the lock transition is slow.
    act(() => {
      vi.advanceTimersByTime(65 * 1000)
    })
    expect(onIdle).toHaveBeenCalledTimes(1)
  })
})
