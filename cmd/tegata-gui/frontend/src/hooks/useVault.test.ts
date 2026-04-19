import { describe, expect, it, beforeEach, vi } from "vitest"
import { renderHook, act, waitFor } from "@testing-library/react"
import { useVault } from "./useVault"
import { App, EventsOn, EventsOff } from "@/lib/wails"

describe("useVault", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("sets view to 'setup' when no vaults found", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([])
    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("setup"))
  })

  it("sets view to 'unlock' and vaultPath when vaults found", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([
      { path: "/usb/vault.tegata", driveName: "USB" },
    ])
    const { result } = renderHook(() => useVault())
    await waitFor(() => {
      expect(result.current.view).toBe("unlock")
      expect(result.current.vaultPath).toBe("/usb/vault.tegata")
    })
  })

  it("sets view to 'setup' when ScanForVaults rejects", async () => {
    vi.mocked(App.ScanForVaults).mockRejectedValue(new Error("scan failed"))
    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("setup"))
  })

  it("sets view to 'setup' when ScanForVaults resolves null", async () => {
    // Simulate Go backend returning null for what TS expects as an array
    vi.mocked(App.ScanForVaults).mockResolvedValue(null as unknown as [])
    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("setup"))
  })

  it("unlock success transitions view to 'main'", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([
      { path: "/usb/vault.tegata", driveName: "USB" },
    ])
    vi.mocked(App.UnlockVault).mockResolvedValue(undefined)

    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("unlock"))

    await act(async () => {
      await result.current.unlock("test-passphrase-dummy")
    })

    expect(result.current.view).toBe("main")
    expect(App.UnlockVault).toHaveBeenCalledTimes(1)
  })

  it("unlock failure sets error and stays on 'unlock'", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([
      { path: "/usb/vault.tegata", driveName: "USB" },
    ])
    vi.mocked(App.UnlockVault).mockRejectedValue(new Error("wrong passphrase"))

    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("unlock"))

    await act(async () => {
      await result.current.unlock("test-passphrase-dummy")
    })

    expect(result.current.view).toBe("unlock")
    expect(result.current.error).toBe("Incorrect passphrase. Please try again.")
  })

  it("lock calls App.LockVault and transitions to 'unlock'", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([
      { path: "/usb/vault.tegata", driveName: "USB" },
    ])
    vi.mocked(App.UnlockVault).mockResolvedValue(undefined)
    vi.mocked(App.LockVault).mockResolvedValue(undefined)

    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("unlock"))

    await act(async () => {
      await result.current.unlock("test-passphrase-dummy")
    })
    expect(result.current.view).toBe("main")

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.view).toBe("unlock")
    expect(App.LockVault).toHaveBeenCalledTimes(1)
  })

  it("createVault success calls CreateVault and UnlockVault, returns recovery key", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([])
    vi.mocked(App.CreateVault).mockResolvedValue("recovery-key-xyz")
    vi.mocked(App.UnlockVault).mockResolvedValue(undefined)

    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("setup"))

    let recoveryKey: string | undefined
    await act(async () => {
      recoveryKey = await result.current.createVault(
        "/usb/vault.tegata",
        "test-passphrase-dummy",
      )
    })

    expect(recoveryKey).toBe("recovery-key-xyz")
    expect(App.CreateVault).toHaveBeenCalledTimes(1)
    expect(App.UnlockVault).toHaveBeenCalledTimes(1)
  })

  it("createVault failure sets error and throws", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([])
    vi.mocked(App.CreateVault).mockRejectedValue(new Error("disk full"))

    const { result } = renderHook(() => useVault())
    await waitFor(() => expect(result.current.view).toBe("setup"))

    await act(async () => {
      await expect(
        result.current.createVault("/usb/vault.tegata", "test-passphrase-dummy"),
      ).rejects.toThrow("disk full")
    })

    expect(result.current.error).toBe("disk full")
  })

  it("registers vault:locked event handler", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([])
    renderHook(() => useVault())
    await waitFor(() => {
      expect(EventsOn).toHaveBeenCalledWith("vault:locked", expect.any(Function))
    })
  })

  it("cleans up vault:locked listener on unmount", async () => {
    vi.mocked(App.ScanForVaults).mockResolvedValue([])
    const { unmount } = renderHook(() => useVault())
    await waitFor(() => {
      expect(EventsOn).toHaveBeenCalled()
    })
    unmount()
    expect(EventsOff).toHaveBeenCalledWith("vault:locked")
  })
})
