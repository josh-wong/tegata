import { type FormEvent, useEffect, useRef, useState } from "react"
import { ArrowLeft } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { LoadingSpinner } from "@/components/shared/LoadingSpinner"
import { EventsOn, EventsOff } from "@/lib/wails"
import type { VaultLocation } from "@/lib/types"

interface UnlockViewProps {
  vaultPath: string | null
  vaultLocations: VaultLocation[]
  error: string | null
  loading: boolean
  onUnlock: (passphrase: string) => void
  onSelectVault: (path: string) => void
  onBack: () => void
}

export function UnlockView({
  vaultPath,
  vaultLocations,
  error,
  loading,
  onUnlock,
  onSelectVault,
  onBack,
}: UnlockViewProps) {
  const [passphrase, setPassphrase] = useState("")
  const [auditStatus, setAuditStatus] = useState("")
  const [errorDismissed, setErrorDismissed] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    EventsOn("audit:unlock-progress", (msg) => setAuditStatus(String(msg)))
    return () => EventsOff("audit:unlock-progress")
  }, [])

  useEffect(() => {
    if (!loading) setAuditStatus("")
  }, [loading])

  // Reset dismissal whenever a new error arrives so it becomes visible again.
  useEffect(() => {
    if (error) setErrorDismissed(false)
  }, [error])

  useEffect(() => {
    // The Wails WebView may not accept focus until its layout is fully
    // interactive. Poll until the input receives focus or give up after
    // a reasonable number of attempts.
    let attempts = 0
    const interval = setInterval(() => {
      if (document.activeElement === inputRef.current || attempts >= 20) {
        clearInterval(interval)
        return
      }
      inputRef.current?.focus()
      attempts++
    }, 100)
    return () => clearInterval(interval)
  }, [])

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!passphrase || loading) return
    onUnlock(passphrase)
  }

  const currentLocation = vaultLocations.find((v) => v.path === vaultPath)

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-md space-y-6">
        <Button
          variant="ghost"
          size="sm"
          onClick={onBack}
          className="gap-1"
        >
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>

        <div className="text-center">
          <h1 className="text-2xl font-bold text-primary">Tegata</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Unlock your vault to continue
          </p>
        </div>

        {vaultLocations.length > 1 && (
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Vault</label>
            <select
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={vaultPath ?? ""}
              onChange={(e) => onSelectVault(e.target.value)}
            >
              {vaultLocations.map((loc) => (
                <option key={loc.path} value={loc.path}>
                  {loc.driveName} — {loc.path}
                </option>
              ))}
            </select>
          </div>
        )}

        {vaultLocations.length === 1 && currentLocation && (
          <p className="text-center text-xs text-muted-foreground">
            {currentLocation.driveName}
          </p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Input
              ref={inputRef}
              type="password"
              placeholder="Passphrase"
              value={passphrase}
              onChange={(e) => { setPassphrase(e.target.value); setErrorDismissed(true) }}
              maxLength={256}
              autoFocus
              disabled={loading}
            />
            {error && !errorDismissed && (
              <p className="text-sm text-destructive">{error}</p>
            )}
          </div>

          <Button
            type="submit"
            className="w-full"
            disabled={!passphrase || loading}
          >
            {loading ? (
              <LoadingSpinner size="sm" />
            ) : (
              "Unlock"
            )}
          </Button>

          {loading && auditStatus && (
            <p className="text-center text-xs text-muted-foreground">
              {auditStatus}
            </p>
          )}
        </form>
      </div>
    </div>
  )
}
