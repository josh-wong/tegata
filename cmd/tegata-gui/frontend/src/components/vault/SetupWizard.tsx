import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
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
  const { t } = useTranslation()
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
  // Debounced so the IPC call isn't fired on every keystroke.
  const removableCheckTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (selectedPath !== "__custom__" || !customPath) return
    if (removableCheckTimer.current) clearTimeout(removableCheckTimer.current)
    removableCheckTimer.current = setTimeout(() => {
      App.IsRemovablePath(customPath)
        .then((isRemovable) => setIsCustomPathRemovable(isRemovable))
        .catch((err) => {
          console.error("Failed to check if path is removable:", err)
          setIsCustomPathRemovable(false) // Assume non-removable on error
        })
    }, 300)
    return () => {
      if (removableCheckTimer.current) clearTimeout(removableCheckTimer.current)
    }
  }, [customPath, selectedPath])

  const folderPath = selectedPath === "__custom__" ? customPath : selectedPath
  const effectivePath = folderPath
    ? `${folderPath.replace(/[/\\]+$/, "")}/${vaultName}.tegata`
    : ""

  async function handleCreate() {
    setValidationError("")
    if (passphrase.length < 8) {
      setValidationError(t("gui.setup.errorPassTooShort"))
      return
    }
    if (passphrase !== confirm) {
      setValidationError(t("gui.setup.errorPassNoMatch"))
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
    <div className="relative z-0 flex min-h-screen items-center justify-center bg-background p-4">
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
            <ArrowLeft className="h-4 w-4" /> {t("gui.common.back")}
          </Button>
        )}

        {/* Step 1: Welcome */}
        {step === 1 && (
          <div className="space-y-6 text-center">
            <div>
              <h1 className="text-2xl font-bold text-primary">Tegata</h1>
              <p className="mt-2 text-muted-foreground">
                {t("gui.setup.step1Subtitle")}
              </p>
            </div>
            <p className="text-sm text-muted-foreground">
              {t("gui.setup.step1Description")}
            </p>
            <Button className="w-full" onClick={() => setStep(2)}>
              {t("gui.setup.createNew")}
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={() => setStep(6 as Step)}
            >
              {t("gui.setup.openExisting")}
            </Button>
            {onCancel && (
              <Button
                variant="ghost"
                className="w-full"
                onClick={onCancel}
              >
                {t("gui.common.cancel")}
              </Button>
            )}
          </div>
        )}

        {/* Step 2: Location picker */}
        {step === 2 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">{t("gui.setup.step2Title")}</h2>
            <p className="text-sm text-muted-foreground">
              <span className="font-semibold text-green-600 dark:text-green-400">{t("gui.setup.step2Tip")}</span>
            </p>

            <div className="space-y-2">
              {removableDrives.map((folder) => (
                <button
                  key={folder.path}
                  onClick={() => { setSelectedPath(folder.path); setCustomPath("") }}
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
                  {t("gui.setup.onlyRemovable")}
                </p>
              )}
            </div>

            <label className="text-sm font-medium">
              {removableDrives.length > 0
                ? t("gui.setup.customPathLabelWithDrives")
                : t("gui.setup.customPathLabelWithoutDrives")}
            </label>
            <Input
              placeholder={t("gui.setup.customPathPlaceholder")}
              value={customPath}
              onChange={(e) => {
                setCustomPath(e.target.value)
                if (selectedPath !== "__custom__") setSelectedPath("__custom__")
              }}
              autoFocus={removableDrives.length === 0}
            />

            {selectedPath === "__custom__" && customPath && !isCustomPathRemovable && (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm">
                <p className="font-medium text-amber-900">{t("gui.setup.warningTitle")}</p>
                <p className="mt-1 text-amber-800">
                  {t("gui.setup.warningBody")}
                </p>
              </div>
            )}

            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("gui.setup.vaultNameLabel")}</label>
              <div className="flex items-stretch gap-0">
                <Input
                  value={vaultName}
                  onChange={(e) => setVaultName(e.target.value.replace(/[^a-zA-Z0-9_-]/g, ""))}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && folderPath && vaultName) {
                      e.preventDefault()
                      setStep(3)
                    }
                  }}
                  className="rounded-r-none"
                  placeholder={t("gui.setup.vaultNamePlaceholder")}
                />
                <span className="flex items-center rounded-r-md border border-l-0 border-input bg-muted px-3 text-sm text-muted-foreground">
                  .tegata
                </span>
              </div>
            </div>

            <Button
              className="w-full"
              disabled={!folderPath || !vaultName}
              onClick={() => setStep(3)}
            >
              {t("gui.setup.continue")}
            </Button>
          </div>
        )}

        {/* Step 3: Passphrase */}
        {step === 3 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">{t("gui.setup.step3Title")}</h2>
            <p className="text-sm text-muted-foreground">
              {t("gui.setup.step3Description")}
            </p>

            <div className="space-y-3">
              <div className="space-y-1.5">
                <Input
                  type="password"
                  placeholder={t("gui.setup.passphrasePlaceholder")}
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  maxLength={256}
                  autoFocus
                />
                {passphrase && <StrengthMeter passphrase={passphrase} />}
              </div>

              <Input
                type="password"
                placeholder={t("gui.setup.confirmPassphrasePlaceholder")}
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
                <LoadingSpinner size="sm" message={t("gui.setup.creating")} />
              ) : (
                t("gui.setup.createVault")
              )}
            </Button>
          </div>
        )}

        {/* Step 4: Recovery key */}
        {step === 4 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">{t("gui.setup.step4Title")}</h2>
            <p className="text-sm text-destructive font-medium">
              {t("gui.setup.step4Warning")}
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
              {t("gui.setup.savedKey")}
            </label>

            <Button
              className="w-full"
              disabled={!savedKey}
              onClick={() => setStep(5)}
            >
              {t("gui.setup.continue")}
            </Button>
          </div>
        )}

        {/* Step 5: Vault created */}
        {step === 5 && (
          <div className="space-y-4 text-center">
            <h2 className="text-xl font-medium">{t("gui.setup.step5Title")}</h2>
            <p className="text-sm text-muted-foreground">
              {t("gui.setup.step5Description")}
            </p>
            <label className={`flex items-center justify-center gap-2 text-sm ${auditLoading ? "opacity-50" : ""}`}>
              <input
                type="checkbox"
                checked={auditOptIn}
                onChange={(e) => { setAuditOptIn(e.target.checked); setAuditError("") }}
                disabled={auditLoading}
                className="rounded border-input"
              />
              {t("gui.setup.enableAudit")}
            </label>
            <p className="text-xs text-muted-foreground">
              {t("gui.setup.auditDescription")}
            </p>
            {auditError && (
              <div className="space-y-2">
                <p className="text-sm text-destructive">{t("gui.setup.auditSetupFailed", { error: auditError })}</p>
                <Button variant="outline" className="w-full" onClick={onComplete}>
                  {t("gui.setup.continueWithoutAudit")}
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
                    {auditProgress || t("gui.setup.settingUpAudit")}
                  </span>
                ) : (
                  t("gui.setup.openVault")
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
              <ArrowLeft className="h-4 w-4" /> {t("gui.common.back")}
            </Button>

            <h2 className="text-lg font-semibold">{t("gui.setup.step6Title")}</h2>

            {existingVaults.length > 0 && (
              <>
                <p className="text-sm text-muted-foreground">
                  {t("gui.setup.detectedVaults")}
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
                ? t("gui.setup.enterPathManually")
                : t("gui.setup.enterPathNoVaults")}
            </p>

            <Input
              placeholder={t("gui.setup.openVaultPlaceholder")}
              value={customPath}
              onChange={(e) => setCustomPath(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && customPath) {
                  e.preventDefault()
                  onOpenExisting(customPath)
                }
              }}
              autoFocus={existingVaults.length === 0}
            />

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button
              className="w-full"
              disabled={!customPath}
              onClick={() => onOpenExisting(customPath)}
            >
              {t("gui.setup.openVault")}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
