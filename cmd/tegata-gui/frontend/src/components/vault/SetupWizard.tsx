import { useState } from "react"
import { ArrowLeft, Copy, Check } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { LoadingSpinner } from "@/components/shared/LoadingSpinner"
import { StrengthMeter } from "@/components/shared/StrengthMeter"
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
  const [selectedPath, setSelectedPath] = useState("")
  const [customPath, setCustomPath] = useState("")
  const [passphrase, setPassphrase] = useState("")
  const [confirm, setConfirm] = useState("")
  const [recoveryKey, setRecoveryKey] = useState("")
  const [savedKey, setSavedKey] = useState(false)
  const [copied, setCopied] = useState(false)
  const [validationError, setValidationError] = useState("")

  const locations = vaultLocations ?? []
  const effectivePath = selectedPath === "__custom__" ? customPath : selectedPath

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
                Set up your portable authenticator
              </p>
            </div>
            <p className="text-sm text-muted-foreground">
              Tegata stores your credentials in an encrypted vault on a USB drive
              or folder of your choice.
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
              Select where to store your encrypted vault.
              {locations.length === 0 &&
                " No removable drives detected — enter a path below."}
            </p>

            <div className="space-y-2">
              {locations.map((loc) => (
                <button
                  key={loc.path}
                  onClick={() => setSelectedPath(loc.path)}
                  className={cn(
                    "w-full rounded-lg border p-3 text-left transition-colors",
                    selectedPath === loc.path
                      ? "border-primary bg-primary/5"
                      : "border-border hover:border-primary/50",
                  )}
                >
                  <div className="font-medium">{loc.driveName}</div>
                  <div className="text-xs text-muted-foreground">{loc.path}</div>
                </button>
              ))}

              <button
                onClick={() => setSelectedPath("__custom__")}
                className={cn(
                  "w-full rounded-lg border border-dashed p-3 text-left transition-colors",
                  selectedPath === "__custom__"
                    ? "border-primary bg-primary/5"
                    : "border-border hover:border-primary/50",
                )}
              >
                <div className="font-medium">Enter a custom path</div>
                <div className="text-xs text-muted-foreground">
                  Specify the full path for your vault file
                </div>
              </button>
            </div>

            {(selectedPath === "__custom__" || locations.length === 0) && (
              <Input
                placeholder="C:\path\to\folder or vault.tegata"
                value={customPath}
                onChange={(e) => {
                  setCustomPath(e.target.value)
                  if (selectedPath !== "__custom__") setSelectedPath("__custom__")
                }}
                autoFocus={locations.length === 0}
              />
            )}

            <Button
              className="w-full"
              disabled={!effectivePath}
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
                  autoFocus
                />
                {passphrase && <StrengthMeter passphrase={passphrase} />}
              </div>

              <Input
                type="password"
                placeholder="Confirm passphrase"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
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
          <div className="space-y-6 text-center">
            <div>
              <h2 className="text-lg font-semibold">Vault created</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Your encrypted vault is ready.
              </p>
            </div>

            <Button className="w-full" onClick={onComplete}>
              Open vault
            </Button>
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
            <p className="text-sm text-muted-foreground">
              Enter the path to your vault file or the folder containing it.
            </p>

            <Input
              placeholder="C:\path\to\vault.tegata"
              value={customPath}
              onChange={(e) => setCustomPath(e.target.value)}
              autoFocus
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
