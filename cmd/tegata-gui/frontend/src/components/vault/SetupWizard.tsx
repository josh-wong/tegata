import { useEffect, useState } from "react"
import { ArrowLeft, Copy, Check } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { LoadingSpinner } from "@/components/shared/LoadingSpinner"
import { StrengthMeter } from "@/components/shared/StrengthMeter"
import { App, EventsOn, EventsOff } from "@/lib/wails"
import type { VaultLocation } from "@/lib/types"
import { cn } from "@/lib/utils"

interface SetupWizardProps {
  vaultLocations: VaultLocation[]
  loading: boolean
  error: string | null
  initialStep?: Step
  onCancel?: () => void
  onCreateVault: (path: string, passphrase: string) => Promise<string>
  onOpenExisting: (path: string) => void
  onComplete: () => void
}

type Step = 1 | 2 | 3 | 4 | 5 | 6


export function SetupWizard({
  vaultLocations,
  loading,
  error,
  initialStep,
  onCancel,
  onCreateVault,
  onOpenExisting,
  onComplete,
}: SetupWizardProps) {
  const [step, setStep] = useState<Step>(initialStep ?? 1)
  const [removableDrives, setRemovableDrives] = useState<VaultLocation[]>([])
  const [selectedPath, setSelectedPath] = useState("")
  const [customPath, setCustomPath] = useState("")
  const [vaultName, setVaultName] = useState("vault")
  const [passphrase, setPassphrase] = useState("")
  const [confirm, setConfirm] = useState("")
  const [recoveryKey, setRecoveryKey] = useState("")
  const [savedKey, setSavedKey] = useState(false)
  const [copied, setCopied] = useState(false)
  const [validationError, setValidationError] = useState("")

  const [existingVaults, setExistingVaults] = useState<VaultLocation[]>(vaultLocations ?? [])
  const [auditOptIn, setAuditOptIn] = useState(false)
  const [auditLoading, setAuditLoading] = useState(false)
  const [auditError, setAuditError] = useState("")
  const [auditProgress, setAuditProgress] = useState("")
  const [isCustomPathRemovable, setIsCustomPathRemovable] = useState(true)

  // Fetch removable drives when entering step 2 (vault creation).
  useEffect(() => {
    if (step === 2) {
      App.ScanRemovableDrives()
        .then((drives) => {
          const d = drives ?? []
          setRemovableDrives(d)
          if (d.length > 0 && !selectedPath) {
            setSelectedPath(d[0].path)
          }
        })
        .catch((err) => console.error("Failed to scan removable drives:", err))
    }
  }, [step]) // eslint-disable-line react-hooks/exhaustive-deps

  // Scan for existing vault files when entering step 6 (open existing).
  useEffect(() => {
    if (step === 6) {
      App.ScanForVaults()
        .then((vaults) => setExistingVaults(vaults ?? []))
        .catch((err) => console.error("Failed to scan for vaults:", err))
    }
  }, [step])

  // Check if the custom path is on a removable drive when it changes.
  useEffect(() => {
    if (selectedPath === "__custom__" && customPath) {
      App.IsRemovablePath(customPath)
        .then((isRemovable) => setIsCustomPathRemovable(isRemovable))
        .catch((err) => {
          console.error("Failed to check if path is removable:", err)
          setIsCustomPathRemovable(false) // Assume non-removable on error
        })
    }
  }, [customPath, selectedPath])

  const folderPath = selectedPath === "__custom__" ? customPath : selectedPath
  const effectivePath = folderPath
    ? `${folderPath.replace(/[/\\]+$/, "")}/${vaultName}.tegata`
    : ""

  async function handleCreate() {
    setValidationError("")
    if (passphrase.length < 8) {
      setValidationError("Passphrase must be at least 8 characters")
      return
    }
    if (passphrase !== confirm) {
      setValidationError("Passphrases do not match")
      return
    }
    try {
      const key = await onCreateVault(effectivePath, passphrase)
      setRecoveryKey(key)
      setPassphrase("")
      setConfirm("")
      setStep(4)
    } catch {
      // Error is surfaced via the error prop
    }
  }

  function handleCopyKey() {
    navigator.clipboard.writeText(recoveryKey).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-md space-y-6">
        {/* Step indicator (only for create flow) */}
        {step !== 6 && (
          <div className="flex items-center justify-center gap-2">
            {([1, 2, 3, 4, 5] as const).map((s) => (
              <div
                key={s}
                className={cn(
                  "h-2.5 w-2.5 rounded-full transition-colors",
                  s === step ? "bg-primary" : s < step ? "bg-primary/50" : "bg-border ring-1 ring-muted-foreground/25",
                )}
              />
            ))}
          </div>
        )}

        {/* Back button */}
        {step > 1 && step < 5 && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setStep((step - 1) as Step)}
            className="gap-1"
          >
            <ArrowLeft className="h-4 w-4" /> Back
          </Button>
        )}

        {/* Step 1: Welcome */}
        {step === 1 && (
          <div className="space-y-6 text-center">
            <div>
              <h1 className="text-2xl font-bold text-primary">Tegata</h1>
              <p className="mt-2 text-muted-foreground">
                Your two-factor authentication codes, encrypted <span className="whitespace-nowrap">and portable</span>
              </p>
            </div>
            <p className="text-sm text-muted-foreground">
              Tegata is a portable authenticator that stores your two-factor
              authentication codes in an encrypted vault.
            </p>
            <Button className="w-full" onClick={() => setStep(2)}>
              Create new vault
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={() => setStep(6 as Step)}
            >
              Open existing vault
            </Button>
            {onCancel && (
              <Button
                variant="ghost"
                className="w-full"
                onClick={onCancel}
              >
                Cancel
              </Button>
            )}
          </div>
        )}

        {/* Step 2: Location picker */}
        {step === 2 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Choose a location</h2>
            <p className="text-sm text-muted-foreground">
              <span className="font-semibold text-green-600">💡 Tip:</span> Store your vault on a USB or microSD for security and portability. Install Tegata on any device to access it.
            </p>

            <div className="space-y-2">
              {removableDrives.map((folder) => (
                <button
                  key={folder.path}
                  onClick={() => setSelectedPath(folder.path)}
                  className={cn(
                    "w-full rounded-lg border p-3 text-left transition-colors",
                    selectedPath === folder.path
                      ? "border-primary bg-primary/5"
                      : "border-border hover:border-primary/50",
                  )}
                >
                  <div className="font-medium">{folder.driveName}</div>
                  <div className="text-xs text-muted-foreground">{folder.path}</div>
                </button>
              ))}

              {removableDrives.length > 0 && (
                <p className="px-1 text-xs text-muted-foreground">
                  Only removable drives (USB, SD, and microSD) are shown.
                </p>
              )}

              <button
                onClick={() => setSelectedPath("__custom__")}
                className={cn(
                  "w-full rounded-lg border border-dashed p-3 text-left transition-colors",
                  selectedPath === "__custom__"
                    ? "border-primary bg-primary/5"
                    : "border-border hover:border-primary/50",
                )}
              >
                <div className="font-medium">Enter a custom folder</div>
              </button>
            </div>

            {(selectedPath === "__custom__" || removableDrives.length === 0) && (
              <Input
                placeholder="C:\path\to\folder"
                value={customPath}
                onChange={(e) => {
                  setCustomPath(e.target.value)
                  if (selectedPath !== "__custom__") setSelectedPath("__custom__")
                }}
                autoFocus={removableDrives.length === 0}
              />
            )}

            {selectedPath === "__custom__" && customPath && !isCustomPathRemovable && (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm">
                <p className="font-medium text-amber-900">⚠️ System drive detected</p>
                <p className="mt-1 text-amber-800">
                  This path is on your computer's main drive. For better security, store your vault on a removable drive like a USB or microSD card — the physical separation keeps your vault safe if your computer is compromised.
                </p>
              </div>
            )}

            <div className="space-y-1.5">
              <label className="text-sm font-medium">Vault name</label>
              <div className="flex items-center gap-0">
                <Input
                  value={vaultName}
                  onChange={(e) => setVaultName(e.target.value.replace(/[^a-zA-Z0-9_-]/g, ""))}
                  className="rounded-r-none"
                  placeholder="vault"
                />
                <span className="flex h-9 items-center rounded-r-md border border-l-0 border-input bg-muted px-3 text-sm text-muted-foreground">
                  .tegata
                </span>
              </div>
            </div>

            <Button
              className="w-full"
              disabled={!folderPath || !vaultName}
              onClick={() => setStep(3)}
            >
              Continue
            </Button>
          </div>
        )}

        {/* Step 3: Passphrase */}
        {step === 3 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Create a passphrase</h2>
            <p className="text-sm text-muted-foreground">
              This passphrase encrypts your vault. Choose something strong and
              memorable.
            </p>

            <div className="space-y-3">
              <div className="space-y-1.5">
                <Input
                  type="password"
                  placeholder="Passphrase"
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  maxLength={256}
                  autoFocus
                />
                {passphrase && <StrengthMeter passphrase={passphrase} />}
              </div>

              <Input
                type="password"
                placeholder="Confirm passphrase"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                maxLength={256}
              />
            </div>

            {(validationError || error) && (
              <p className="text-sm text-destructive">
                {validationError || error}
              </p>
            )}

            <Button
              className="w-full"
              disabled={!passphrase || !confirm || loading}
              onClick={handleCreate}
            >
              {loading ? (
                <LoadingSpinner size="sm" message="Creating vault..." />
              ) : (
                "Create vault"
              )}
            </Button>
          </div>
        )}

        {/* Step 4: Recovery key */}
        {step === 4 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Save your recovery key</h2>
            <p className="text-sm text-destructive font-medium">
              Store this key separately from your USB drive. It is the only way
              to recover your vault if you forget your passphrase.
            </p>

            <div className="relative rounded-lg bg-muted p-4">
              <code className="block break-all font-mono text-sm">
                {recoveryKey}
              </code>
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-2 top-2 h-7 w-7"
                onClick={handleCopyKey}
              >
                {copied ? (
                  <Check className="h-4 w-4 text-green-500" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>

            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={savedKey}
                onChange={(e) => setSavedKey(e.target.checked)}
                className="rounded border-input"
              />
              I have saved my recovery key
            </label>

            <Button
              className="w-full"
              disabled={!savedKey}
              onClick={() => setStep(5)}
            >
              Continue
            </Button>
          </div>
        )}

        {/* Step 5: Vault created */}
        {step === 5 && (
          <div className="space-y-4 text-center">
            <h2 className="text-xl font-medium">Vault created</h2>
            <p className="text-sm text-muted-foreground">
              Your vault has been created and encrypted. Save your recovery key somewhere safe.
            </p>
            <label className={`flex items-center justify-center gap-2 text-sm ${auditLoading ? "opacity-50" : ""}`}>
              <input
                type="checkbox"
                checked={auditOptIn}
                onChange={(e) => { setAuditOptIn(e.target.checked); setAuditError("") }}
                disabled={auditLoading}
                className="rounded border-input"
              />
              Enable audit logging
            </label>
            <p className="text-xs text-muted-foreground">
              Log every authentication event to a tamper-evident ledger. Requires Docker.
            </p>
            {auditError && (
              <div className="space-y-2">
                <p className="text-sm text-destructive">Audit setup failed: {auditError}</p>
                <Button variant="outline" className="w-full" onClick={onComplete}>
                  Continue without audit
                </Button>
              </div>
            )}
            {!auditError && (
              <Button className="w-full" disabled={auditLoading} onClick={async () => {
                if (auditOptIn) {
                  setAuditLoading(true)
                  setAuditError("")
                  setAuditProgress("")
                  EventsOn("audit:progress", (msg) => setAuditProgress(String(msg)))
                  try {
                    await App.StartAuditServer()
                  } catch (err) {
                    setAuditError(err instanceof Error ? err.message : String(err))
                    setAuditLoading(false)
                    EventsOff("audit:progress")
                    return
                  }
                  setAuditLoading(false)
                  EventsOff("audit:progress")
                }
                onComplete()
              }}>
                {auditLoading ? (
                  <span className="flex items-center gap-2">
                    <svg className="animate-spin h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                    {auditProgress || "Setting up audit..."}
                  </span>
                ) : (
                  "Open vault"
                )}
              </Button>
            )}
          </div>
        )}

        {/* Step 6: Open existing vault */}
        {step === 6 && (
          <div className="space-y-4">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setStep(1)}
              className="gap-1"
            >
              <ArrowLeft className="h-4 w-4" /> Back
            </Button>

            <h2 className="text-lg font-semibold">Open existing vault</h2>

            {existingVaults.length > 0 && (
              <>
                <p className="text-sm text-muted-foreground">
                  Detected vaults on your removable drives.
                </p>
                <div className="space-y-2">
                  {existingVaults.map((v) => (
                    <button
                      key={v.path}
                      onClick={() => onOpenExisting(v.path)}
                      className="w-full rounded-lg border p-3 text-left transition-colors border-border hover:border-primary/50"
                    >
                      <div className="font-medium">{v.path.split(/[/\\]/).pop()}</div>
                      <div className="text-xs text-muted-foreground">{v.path}</div>
                    </button>
                  ))}
                </div>
              </>
            )}

            <p className="text-sm text-muted-foreground">
              {existingVaults.length > 0
                ? "Or enter a path manually."
                : "Enter the path to your vault file."}
            </p>

            <Input
              placeholder="C:\path\to\vault.tegata"
              value={customPath}
              onChange={(e) => setCustomPath(e.target.value)}
              autoFocus={existingVaults.length === 0}
            />

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button
              className="w-full"
              disabled={!customPath}
              onClick={() => onOpenExisting(customPath)}
            >
              Open vault
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
