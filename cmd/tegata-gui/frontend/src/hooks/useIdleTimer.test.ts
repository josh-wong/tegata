import { describe, expect, it, beforeEach, afterEach, vi } from "vitest"
import { renderHook, act } from "@testing-library/react"
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
})
