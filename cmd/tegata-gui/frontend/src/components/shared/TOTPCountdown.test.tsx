import { describe, expect, it, beforeEach, afterEach, vi } from "vitest"
import { render, screen, act } from "@testing-library/react"
import { TOTPCountdown } from "./TOTPCountdown"

describe("TOTPCountdown", () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it("renders the 6-digit code formatted with space", () => {
    render(
      <TOTPCountdown code="123456" remaining={15} period={30} onExpired={vi.fn()} />,
    )
    expect(screen.getByText("123 456")).toBeInTheDocument()
  })

  it("renders the 8-digit code formatted with space", () => {
    render(
      <TOTPCountdown code="12345678" remaining={15} period={30} onExpired={vi.fn()} />,
    )
    expect(screen.getByText("1234 5678")).toBeInTheDocument()
  })

  it("shows remaining seconds count", () => {
    render(
      <TOTPCountdown code="123456" remaining={15} period={30} onExpired={vi.fn()} />,
    )
    expect(screen.getByText("15")).toBeInTheDocument()
  })

  it("decrements seconds on interval tick", () => {
    render(
      <TOTPCountdown code="123456" remaining={5} period={30} onExpired={vi.fn()} />,
    )
    expect(screen.getByText("5")).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText("4")).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText("3")).toBeInTheDocument()
  })

  it("calls onExpired when countdown reaches 0", () => {
    const onExpired = vi.fn()
    render(
      <TOTPCountdown code="123456" remaining={2} period={30} onExpired={onExpired} />,
    )

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(onExpired).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(onExpired).toHaveBeenCalledTimes(1)
  })

  it("cleans up interval on unmount — onExpired not called again", () => {
    const onExpired = vi.fn()
    const { unmount } = render(
      <TOTPCountdown code="123456" remaining={3} period={30} onExpired={onExpired} />,
    )

    unmount()

    act(() => {
      vi.advanceTimersByTime(10000)
    })

    expect(onExpired).not.toHaveBeenCalled()
  })
})
