import { describe, expect, it, beforeEach, vi } from "vitest"
import { renderHook, act } from "@testing-library/react"
import { useTheme } from "./useTheme"

describe("useTheme", () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove("dark")
    vi.restoreAllMocks()
  })

  it("defaults to 'system' when localStorage is empty", () => {
    const { result } = renderHook(() => useTheme())
    expect(result.current.theme).toBe("system")
  })

  it("reads stored theme from localStorage on init", () => {
    localStorage.setItem("tegata-theme", "dark")
    const { result } = renderHook(() => useTheme())
    expect(result.current.theme).toBe("dark")
  })

  it("setTheme updates localStorage and toggles 'dark' class", () => {
    const { result } = renderHook(() => useTheme())

    act(() => {
      result.current.setTheme("dark")
    })

    expect(localStorage.getItem("tegata-theme")).toBe("dark")
    expect(document.documentElement.classList.contains("dark")).toBe(true)

    act(() => {
      result.current.setTheme("light")
    })

    expect(localStorage.getItem("tegata-theme")).toBe("light")
    expect(document.documentElement.classList.contains("dark")).toBe(false)
  })

  it("system theme applies based on matchMedia", () => {
    // Default matchMedia mock returns matches: false (light).
    const { result } = renderHook(() => useTheme())

    act(() => {
      result.current.setTheme("system")
    })

    // matchMedia default is matches: false -> light -> no dark class
    expect(document.documentElement.classList.contains("dark")).toBe(false)
  })

  it("registers change listener on matchMedia for system theme updates", () => {
    const addEventListener = vi.fn()
    const removeEventListener = vi.fn()

    vi.mocked(window.matchMedia).mockReturnValue({
      matches: false,
      media: "(prefers-color-scheme: dark)",
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener,
      removeEventListener,
      dispatchEvent: vi.fn(),
    })

    const { unmount } = renderHook(() => useTheme())

    expect(addEventListener).toHaveBeenCalledWith("change", expect.any(Function))

    unmount()
    expect(removeEventListener).toHaveBeenCalledWith("change", expect.any(Function))
  })

  it("matchMedia change handler applies system theme when theme is 'system'", () => {
    let capturedHandler: (() => void) | null = null

    vi.mocked(window.matchMedia).mockReturnValue({
      matches: false,
      media: "(prefers-color-scheme: dark)",
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn((_event: string, handler: () => void) => {
        capturedHandler = handler
      }),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })

    // Theme defaults to "system" when localStorage is empty
    renderHook(() => useTheme())

    expect(capturedHandler).not.toBeNull()

    // Simulate OS switching to dark mode
    vi.mocked(window.matchMedia).mockReturnValue({
      matches: true,
      media: "(prefers-color-scheme: dark)",
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })

    act(() => {
      capturedHandler!()
    })

    expect(document.documentElement.classList.contains("dark")).toBe(true)
  })
})
