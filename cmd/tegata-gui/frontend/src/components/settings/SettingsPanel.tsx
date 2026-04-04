import { useEffect, useRef, useState } from "react"
import { Info, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { StrengthMeter } from "@/components/shared/StrengthMeter"
import { useTheme } from "@/hooks/useTheme"
import { App } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"
import { cn, formatError } from "@/lib/utils"

interface SettingsPanelProps {
  open: boolean
  onClose: () => void
  onCredentialsChanged: () => void
  updateInfo: UpdateInfo | null
}

export function SettingsPanel({ open, onClose, onCredentialsChanged, updateInfo }: SettingsPanelProps) {
  const { theme, setTheme } = useTheme()

  const [showPassChange, setShowPassChange] = useState(false)
  const [currentPass, setCurrentPass] = useState("")
  const [newPass, setNewPass] = useState("")
  const [confirmPass, setConfirmPass] = useState("")
  const [passError, setPassError] = useState("")
  const [passSuccess, setPassSuccess] = useState(false)

  const [showRecovery, setShowRecovery] = useState(false)
  const [recoveryKey, setRecoveryKey] = useState("")
  const [recoveryResult, setRecoveryResult] = useState<boolean | null>(null)

  const [idleTimeout, setIdleTimeout] = useState(300)
  const [auditConfigured, setAuditConfigured] = useState(false)
  const [autoStart, setAutoStart] = useState(false)
  const [appVersion, setAppVersion] = useState("")
  const [showTooltip, setShowTooltip] = useState(false)
  const [tooltipPos, setTooltipPos] = useState({ top: 0, left: 0 })
  const infoRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) {
      App.GetIdleTimeoutSeconds()
        .then(setIdleTimeout)
        .catch(() => {})
      App.GetVersion()
        .then(setAppVersion)
        .catch(() => {})
      App.IsAuditConfigured()
        .then((configured) => {
          setAuditConfigured(configured)
          if (configured) {
            App.GetAuditAutoStart()
              .then(setAutoStart)
              .catch(() => {})
          }
        })
        .catch(() => {})
    }
  }, [open])

  async function handleIdleTimeoutChange(seconds: number) {
    setIdleTimeout(seconds)
    try {
      await App.SetIdleTimeoutSeconds(seconds)
    } catch {
      // Revert on failure
      const current = await App.GetIdleTimeoutSeconds()
      setIdleTimeout(current)
    }
  }

  async function handleAutoStartChange(enabled: boolean) {
    const prev = autoStart
    setAutoStart(enabled)
    try {
      await App.SetAuditAutoStart(enabled)
    } catch {
      setAutoStart(prev)
    }
  }

  if (!open) return null

  async function handleChangePassphrase() {
    setPassError("")
    if (newPass.length < 8) {
      setPassError("Passphrase must be at least 8 characters")
      return
    }
    if (newPass !== confirmPass) {
      setPassError("Passphrases do not match")
      return
    }
    try {
      await App.ChangePassphrase(currentPass, newPass)
      setPassSuccess(true)
      setCurrentPass("")
      setNewPass("")
      setConfirmPass("")
      setTimeout(() => { setPassSuccess(false); setShowPassChange(false) }, 2000)
    } catch (err) {
      setPassError(formatError(err, "Failed to change passphrase"))
    }
  }

  async function handleVerifyRecovery() {
    try {
      const valid = await App.VerifyRecoveryKey(recoveryKey)
      setRecoveryResult(valid)
    } catch {
      setRecoveryResult(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg bg-card p-6 shadow-lg max-h-[80vh] overflow-y-auto">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Settings</h2>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Theme */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">Theme</h3>
          <div className="flex gap-1">
            {(["system", "light", "dark"] as const).map((t) => (
              <Button
                key={t}
                variant={theme === t ? "default" : "outline"}
                size="sm"
                onClick={() => setTheme(t)}
                className={cn("flex-1 capitalize", theme === t && "pointer-events-none")}
              >
                {t}
              </Button>
            ))}
          </div>
        </section>

        <Separator className="my-4" />

        {/* Auto-lock */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">Auto-lock</h3>
          <select
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={idleTimeout}
            onChange={(e) => handleIdleTimeoutChange(Number(e.target.value))}
          >
            <option value={60}>1 minute</option>
            <option value={120}>2 minutes</option>
            <option value={300}>5 minutes (default)</option>
            <option value={600}>10 minutes</option>
            <option value={900}>15 minutes</option>
            <option value={1800}>30 minutes</option>
            <option value={0}>Never</option>
          </select>
          <p className="text-xs text-muted-foreground">
            Lock the vault automatically after a period of inactivity.
          </p>
        </section>

        <Separator className="my-4" />

        {/* Vault */}
        <section className="space-y-3">
          <h3 className="text-sm font-medium">Vault</h3>

          {!showPassChange ? (
            <Button variant="outline" size="sm" onClick={() => setShowPassChange(true)}>
              Change passphrase
            </Button>
          ) : (
            <div className="space-y-2 rounded-md border border-border p-3">
              <Input
                type="password"
                placeholder="Current passphrase"
                value={currentPass}
                onChange={(e) => setCurrentPass(e.target.value)}
              />
              <Input
                type="password"
                placeholder="New passphrase"
                value={newPass}
                onChange={(e) => setNewPass(e.target.value)}
              />
              {newPass && <StrengthMeter passphrase={newPass} />}
              <Input
                type="password"
                placeholder="Confirm new passphrase"
                value={confirmPass}
                onChange={(e) => setConfirmPass(e.target.value)}
              />
              {passError && <p className="text-sm text-destructive">{passError}</p>}
              {passSuccess && <p className="text-sm text-green-500">Passphrase changed</p>}
              <div className="flex gap-2">
                <Button size="sm" onClick={handleChangePassphrase}>Save</Button>
                <Button size="sm" variant="outline" onClick={() => {
                  setShowPassChange(false)
                  setCurrentPass("")
                  setNewPass("")
                  setConfirmPass("")
                  setPassError("")
                }}>
                  Cancel
                </Button>
              </div>
            </div>
          )}

          {!showRecovery ? (
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" onClick={() => setShowRecovery(true)}>
                Verify recovery key
              </Button>
              <div
                ref={infoRef}
                className="cursor-help text-muted-foreground"
                onMouseEnter={() => {
                  if (infoRef.current) {
                    const rect = infoRef.current.getBoundingClientRect()
                    setTooltipPos({ top: rect.bottom + 8, left: rect.left + rect.width / 2 })
                  }
                  setShowTooltip(true)
                }}
                onMouseLeave={() => setShowTooltip(false)}
              >
                <Info className="h-4 w-4" />
                {showTooltip && (
                  <div
                    className="fixed z-[100] w-72 -translate-x-1/2 rounded-md bg-neutral-800 px-3 py-2 text-xs text-neutral-100 shadow-md dark:bg-neutral-700"
                    style={{ top: tooltipPos.top, left: tooltipPos.left }}
                  >
                    Confirm that the recovery key you saved still matches your vault. This is the only way to regain access if you forget your passphrase.
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="space-y-2 rounded-md border border-border p-3">
              <Input
                placeholder="Enter recovery key"
                value={recoveryKey}
                onChange={(e) => { setRecoveryKey(e.target.value); setRecoveryResult(null) }}
                className="font-mono"
              />
              {recoveryResult === true && <p className="text-sm text-green-500">Recovery key is valid</p>}
              {recoveryResult === false && (
                <div className="space-y-2 rounded-md bg-destructive/10 p-3">
                  <p className="text-sm font-medium text-destructive">Recovery key is invalid</p>
                  <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
                    <li>Check for typos—dashes, spaces, and letter case matter.</li>
                    <li>Make sure this key is for this vault, not a different one.</li>
                    <li>If you have lost your recovery key, consider changing your passphrase to something memorable while you still have access.</li>
                    <li>You may also want to export your credentials as a backup.</li>
                  </ul>
                </div>
              )}
              <div className="flex gap-2">
                <Button size="sm" onClick={handleVerifyRecovery}>Verify</Button>
                <Button size="sm" variant="outline" onClick={() => {
                  setShowRecovery(false)
                  setRecoveryKey("")
                  setRecoveryResult(null)
                }}>
                  Cancel
                </Button>
              </div>
            </div>
          )}
          <ExportImport onImported={onCredentialsChanged} />
        </section>

        {auditConfigured && (
          <>
            <Separator className="my-4" />
            <section className="space-y-2">
              <h3 className="text-sm font-medium">Audit</h3>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={autoStart}
                  onChange={(e) => handleAutoStartChange(e.target.checked)}
                  className="rounded border-input"
                />
                Auto-start ledger server
              </label>
              <p className="text-xs text-muted-foreground">
                Automatically start the audit ledger when you unlock the vault.
              </p>
            </section>
          </>
        )}

        <Separator className="my-4" />

        {/* Update */}
        {updateInfo && (
          <>
            <section className="space-y-2">
              <h3 className="text-sm font-medium">Update available</h3>
              <p className="text-sm text-muted-foreground">
                Version {updateInfo.version} is available.
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.open(updateInfo.url, "_blank")}
              >
                Download
              </Button>
            </section>
            <Separator className="my-4" />
          </>
        )}

        {/* About */}
        <section className="space-y-1">
          <h3 className="text-sm font-medium">About</h3>
          <p className="text-xs text-muted-foreground">Tegata — Your credentials, encrypted and portable</p>
          {appVersion && <p className="text-xs text-muted-foreground">Version: {appVersion}</p>}
          <p className="text-xs text-muted-foreground">License: MIT</p>
        </section>
      </div>
    </div>
  )
}

function ExportImport({ onImported }: { onImported: () => void }) {
  const [showExport, setShowExport] = useState(false)
  const [showImport, setShowImport] = useState(false)
  const [exportPass, setExportPass] = useState("")
  const [exportConfirm, setExportConfirm] = useState("")
  const [importPass, setImportPass] = useState("")
  const [importFile, setImportFile] = useState<string | null>(null)
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleExport() {
    if (!exportPass) return
    if (exportPass.length < 8) {
      setMessage({ text: "Passphrase must be at least 8 characters", error: true })
      return
    }
    if (exportPass !== exportConfirm) {
      setMessage({ text: "Passphrases do not match", error: true })
      return
    }
    setLoading(true)
    setMessage(null)
    try {
      const path = await App.ExportVaultToFile(exportPass)
      if (path) {
        setMessage({ text: `Exported to ${path}`, error: false })
        setExportPass("")
        setExportConfirm("")
      }
    } catch (err) {
      setMessage({ text: formatError(err, "Export failed"), error: true })
    } finally {
      setLoading(false)
    }
  }

  async function handlePickFile() {
    try {
      const path = await App.PickImportFile()
      if (path) {
        setImportFile(path)
      }
    } catch (err) {
      setMessage({ text: formatError(err, "Failed to select file"), error: true })
    }
  }

  async function handleImport() {
    if (!importPass || !importFile) return
    setLoading(true)
    setMessage(null)
    try {
      const result = await App.ImportVaultFromFile(importFile, importPass)
      if (result) {
        setMessage({ text: `Imported ${result.imported} credential(s), ${result.skipped} skipped`, error: false })
        setImportPass("")
        setImportFile(null)
        onImported()
      }
    } catch (err) {
      setMessage({ text: formatError(err, "Import failed"), error: true })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-2 pt-2">
      {!showExport && !showImport && (
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => { setShowExport(true); setMessage(null) }}>
            Export credentials
          </Button>
          <Button variant="outline" size="sm" onClick={() => { setShowImport(true); setMessage(null) }}>
            Import credentials
          </Button>
        </div>
      )}

      {showExport && (
        <div className="space-y-2 rounded-md border border-border p-3">
          <p className="text-xs text-muted-foreground">
            Enter a passphrase to encrypt the export file.
          </p>
          <Input
            type="password"
            placeholder="Export passphrase"
            value={exportPass}
            onChange={(e) => { setExportPass(e.target.value); setMessage(null) }}
          />
          {exportPass.length > 0 && <StrengthMeter passphrase={exportPass} />}
          <Input
            type="password"
            placeholder="Confirm passphrase"
            value={exportConfirm}
            onChange={(e) => { setExportConfirm(e.target.value); setMessage(null) }}
          />
          {message && (
            <p className={`text-sm ${message.error ? "text-destructive" : "text-green-500"}`}>
              {message.text}
            </p>
          )}
          <div className="flex gap-2">
            <Button size="sm" onClick={handleExport} disabled={!exportPass || !exportConfirm || loading}>
              {loading ? "Exporting..." : "Export to file"}
            </Button>
            <Button size="sm" variant="outline" onClick={() => { setShowExport(false); setExportPass(""); setExportConfirm(""); setMessage(null) }}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {showImport && (
        <div className="space-y-2 rounded-md border border-border p-3">
          {message && !message.error ? (
            <>
              <p className="text-sm text-green-500">{message.text}</p>
              <Button size="sm" variant="outline" onClick={() => { setShowImport(false); setImportPass(""); setImportFile(null); setMessage(null) }}>
                Done
              </Button>
            </>
          ) : (
            <>
              {!importFile ? (
                <>
                  <p className="text-xs text-muted-foreground">
                    Select the encrypted export file to import.
                  </p>
                  <Button size="sm" onClick={handlePickFile}>
                    Choose file
                  </Button>
                </>
              ) : (
                <>
                  <p className="text-xs text-muted-foreground">
                    File: {importFile.split(/[/\\]/).pop()}
                  </p>
                  <Input
                    type="password"
                    placeholder="Passphrase used during export"
                    value={importPass}
                    onChange={(e) => setImportPass(e.target.value)}
                    autoFocus
                  />
                </>
              )}
              {message && (
                <p className="text-sm text-destructive">{message.text}</p>
              )}
              <div className="flex gap-2">
                {importFile && (
                  <Button size="sm" onClick={handleImport} disabled={!importPass || loading}>
                    {loading ? "Importing..." : "Import"}
                  </Button>
                )}
                <Button size="sm" variant="outline" onClick={() => { setShowImport(false); setImportPass(""); setImportFile(null); setMessage(null) }}>
                  Cancel
                </Button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}
