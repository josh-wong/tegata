import { describe, expect, it, beforeEach, vi } from "vitest"
import { renderHook, act } from "@testing-library/react"
import { useCredentials } from "./useCredentials"
import { App } from "@/lib/wails"
import type { Credential } from "@/lib/types"

function makeCredential(overrides: Partial<Credential> = {}): Credential {
  return {
    id: "1",
    label: "GitHub",
    issuer: "GitHub Inc",
    type: "totp",
    algorithm: "SHA1",
    digits: 6,
    period: 30,
    counter: 0,
    tags: [],
    notes: "",
    ...overrides,
  }
}

describe("useCredentials", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("refresh populates credentials from ListCredentials", async () => {
    vi.mocked(App.ListCredentials).mockResolvedValue([
      makeCredential({ tags: ["dev"], notes: "work" }),
    ])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    expect(result.current.credentials).toHaveLength(1)
    expect(result.current.credentials[0].label).toBe("GitHub")
  })

  it("refresh with null response sets empty array", async () => {
    // Simulate Go backend returning null for what TS expects as an array
    vi.mocked(App.ListCredentials).mockResolvedValue(null as unknown as Credential[])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    expect(result.current.credentials).toEqual([])
  })

  it("refresh on error sets empty array", async () => {
    vi.mocked(App.ListCredentials).mockRejectedValue(new Error("network error"))

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    expect(result.current.credentials).toEqual([])
  })

  it("normalizes null tags, issuer, and notes to defaults", async () => {
    vi.mocked(App.ListCredentials).mockResolvedValue([
      {
        id: "2",
        label: "Test",
        issuer: null as unknown as string,
        type: "totp",
        algorithm: null as unknown as string,
        digits: 0,
        period: 0,
        counter: null as unknown as number,
        tags: null as unknown as string[],
        notes: null as unknown as string,
      },
    ])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    const cred = result.current.credentials[0]
    expect(cred.tags).toEqual([])
    expect(cred.issuer).toBe("")
    expect(cred.notes).toBe("")
    expect(cred.algorithm).toBe("SHA1")
    expect(cred.digits).toBe(6)
    expect(cred.period).toBe(30)
    expect(cred.counter).toBe(0)
  })

  it("filteredCredentials filters by label case-insensitively", async () => {
    vi.mocked(App.ListCredentials).mockResolvedValue([
      makeCredential({ id: "1", label: "GitHub", issuer: "GitHub Inc" }),
      makeCredential({ id: "2", label: "Gmail", issuer: "Google" }),
    ])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    act(() => {
      result.current.search("git")
    })

    expect(result.current.filteredCredentials).toHaveLength(1)
    expect(result.current.filteredCredentials[0].label).toBe("GitHub")
  })

  it("filteredCredentials filters by issuer", async () => {
    vi.mocked(App.ListCredentials).mockResolvedValue([
      makeCredential({ id: "1", label: "Work", issuer: "GitHub Inc" }),
      makeCredential({ id: "2", label: "Personal", issuer: "Google" }),
    ])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    act(() => {
      result.current.search("google")
    })

    expect(result.current.filteredCredentials).toHaveLength(1)
    expect(result.current.filteredCredentials[0].id).toBe("2")
  })

  it("selectedCredential returns matching credential or null", async () => {
    vi.mocked(App.ListCredentials).mockResolvedValue([
      makeCredential(),
    ])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    expect(result.current.selectedCredential).toBeNull()

    act(() => {
      result.current.select("1")
    })

    expect(result.current.selectedCredential?.label).toBe("GitHub")

    act(() => {
      result.current.select("nonexistent")
    })

    expect(result.current.selectedCredential).toBeNull()
  })

  it("returns all credentials when search query is empty", async () => {
    vi.mocked(App.ListCredentials).mockResolvedValue([
      makeCredential({ id: "1", label: "A", issuer: "" }),
      makeCredential({ id: "2", label: "B", issuer: "" }),
    ])

    const { result } = renderHook(() => useCredentials())
    await act(async () => {
      await result.current.refresh()
    })

    expect(result.current.filteredCredentials).toHaveLength(2)
  })
})
