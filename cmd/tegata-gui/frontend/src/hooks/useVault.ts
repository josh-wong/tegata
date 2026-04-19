import { useCallback, useEffect, useState } from "react"
import { App, EventsOn, EventsOff } from "@/lib/wails"
import { formatError } from "@/lib/utils"
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
      setView((current) => (current === "main" ? "unlock" : current))
    }
    EventsOn("vault:locked", handler)
    return () => EventsOff("vault:locked")
  }, [])

  const unlock = useCallback(
    async (passphrase: string) => {
      if (!vaultPath) return
      setLoading(true)
      // Must remain here: cycling error through null before each attempt ensures
      // UnlockView's showError state re-triggers even when consecutive failures
      // produce the same error string.
      setError(null)
      try {
        await App.UnlockVault(vaultPath, passphrase)
        setView("main")
      } catch (err) {
        const raw = formatError(err, "")
        // Default to the passphrase error for all failures — most unlock errors
        // are wrong passphrases. Only show the generic vault-access message when
        // we can positively identify a system-level cause (file permissions,
        // missing file, etc.) where a wrong passphrase clearly isn't the issue.
        // TODO: Replace this heuristic with structured error codes from the Go
        // backend so the distinction is reliable and not coupled to error message
        // wording (#63).
        const isSystemError = raw
          ? /permission denied|no such file|access denied|read-only file/i.test(raw)
          : false
        setError(
          isSystemError
            ? "Failed to open vault. Please try again."
            : "Incorrect passphrase. Please try again.",
        )
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
        setError(formatError(err, "Failed to create vault"))
        throw err
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  const clearError = useCallback(() => setError(null), [])

  return {
    view,
    setView,
    isUnlocked: view === "main",
    vaultPath,
    setVaultPath,
    vaultLocations,
    error,
    clearError,
    loading,
    unlock,
    lock,
    createVault,
  } as const
}
