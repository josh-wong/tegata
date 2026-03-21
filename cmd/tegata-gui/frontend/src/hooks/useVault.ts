import { useCallback, useEffect, useState } from "react"
import { App, EventsOn, EventsOff } from "@/lib/wails"
import type { AppView, VaultLocation } from "@/lib/types"

export function useVault() {
  const [view, setViewRaw] = useState<AppView>("loading")
  const [prevView, setPrevView] = useState<AppView>("setup")
  const [vaultPath, setVaultPath] = useState<string | null>(null)

  const setView = useCallback((next: AppView) => {
    setViewRaw((current) => {
      setPrevView(current)
      return next
    })
  }, [])
  const [vaultLocations, setVaultLocations] = useState<VaultLocation[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    App.ScanForVaults()
      .then((locations) => {
        const locs = locations ?? []
        setVaultLocations(locs)
        if (locs.length === 0) {
          setView("setup")
        } else {
          setVaultPath(locs[0].path)
          setView("unlock")
        }
      })
      .catch(() => {
        setView("setup")
      })
  }, [])

  useEffect(() => {
    const handler = () => {
      // Only force unlock view if currently in main (idle timeout lock).
      // Don't override if user is intentionally navigating (e.g. switch vault).
      setViewRaw((current) => (current === "main" ? "unlock" : current))
    }
    EventsOn("vault:locked", handler)
    return () => EventsOff("vault:locked")
  }, [])

  const unlock = useCallback(
    async (passphrase: string) => {
      if (!vaultPath) return
      setLoading(true)
      setError(null)
      try {
        await App.UnlockVault(vaultPath, passphrase)
        setView("main")
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to unlock vault")
      } finally {
        setLoading(false)
      }
    },
    [vaultPath],
  )

  const lock = useCallback(async () => {
    try {
      await App.LockVault()
    } catch {
      // Lock failure is non-critical
    }
    setView("unlock")
  }, [])

  const createVault = useCallback(
    async (path: string, passphrase: string): Promise<string> => {
      setLoading(true)
      setError(null)
      try {
        const recoveryKey = await App.CreateVault(path, passphrase)
        setVaultPath(path)
        // Unlock immediately using the path parameter directly,
        // not vaultPath state which hasn't flushed yet.
        await App.UnlockVault(path, passphrase)
        return recoveryKey
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to create vault")
        throw err
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  return {
    view,
    prevView,
    setView,
    vaultPath,
    setVaultPath,
    vaultLocations,
    error,
    loading,
    unlock,
    lock,
    createVault,
  } as const
}
