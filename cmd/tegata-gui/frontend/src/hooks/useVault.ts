import { useCallback, useEffect, useState } from "react"
import { App, EventsOn, EventsOff } from "@/lib/wails"
import type { AppView, VaultLocation } from "@/lib/types"

export function useVault() {
  const [view, setView] = useState<AppView>("loading")
  const [vaultPath, setVaultPath] = useState<string | null>(null)
  const [vaultLocations, setVaultLocations] = useState<VaultLocation[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    App.ScanForVaults()
      .then((locations) => {
        setVaultLocations(locations)
        if (locations.length === 0) {
          setView("setup")
        } else {
          setVaultPath(locations[0].path)
          setView("unlock")
        }
      })
      .catch(() => {
        setView("setup")
      })
  }, [])

  useEffect(() => {
    const handler = () => {
      setView("unlock")
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
