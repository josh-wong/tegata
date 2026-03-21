import { useState } from "react"
import { Info, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { StrengthMeter } from "@/components/shared/StrengthMeter"
import { useTheme } from "@/hooks/useTheme"
import { App } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"
import { cn } from "@/lib/utils"

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
      setPassError(err instanceof Error ? err.message : "Failed to change passphrase")
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
              <div className="group relative cursor-help text-muted-foreground">
                <Info className="h-4 w-4" />
                <div className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 hidden w-80 -translate-x-1/2 rounded-md bg-foreground px-3 py-2 text-xs text-background shadow-md group-hover:block">
                  Confirm that the recovery key you saved still matches your vault. This is the only way to regain access if you forget your passphrase.
                </div>
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

        <Separator className="my-4" />

        {/* Update */}
        {updateInfo && (
          <>
            <section className="space-y-2">
              <h3 className="text-sm font-medium">Update available</h3>
              <p className="text-sm text-muted-foreground">
                Version {updateInfo.Version} is available.
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.open(updateInfo.URL, "_blank")}
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
          <p className="text-xs text-muted-foreground">Tegata — Portable encrypted authenticator</p>
          <p className="text-xs text-muted-foreground">License: Apache 2.0</p>
        </section>
      </div>
    </div>
  )
}

function ExportImport({ onImported }: { onImported: () => void }) {
  const [showExport, setShowExport] = useState(false)
  const [showImport, setShowImport] = useState(false)
  const [exportPass, setExportPass] = useState("")
  const [importPass, setImportPass] = useState("")
  const [importFile, setImportFile] = useState<string | null>(null)
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleExport() {
    if (!exportPass) return
    setLoading(true)
    setMessage(null)
    try {
      const path = await App.ExportVaultToFile(exportPass)
      if (path) {
        setMessage({ text: `Exported to ${path}`, error: false })
        setExportPass("")
      }
    } catch (err) {
      setMessage({ text: err instanceof Error ? err.message : "Export failed", error: true })
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
      setMessage({ text: err instanceof Error ? err.message : "Failed to select file", error: true })
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
      setMessage({ text: err instanceof Error ? err.message : "Import failed", error: true })
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
            onChange={(e) => setExportPass(e.target.value)}
          />
          {message && (
            <p className={`text-sm ${message.error ? "text-destructive" : "text-green-500"}`}>
              {message.text}
            </p>
          )}
          <div className="flex gap-2">
            <Button size="sm" onClick={handleExport} disabled={!exportPass || loading}>
              {loading ? "Exporting..." : "Export to file"}
            </Button>
            <Button size="sm" variant="outline" onClick={() => { setShowExport(false); setExportPass(""); setMessage(null) }}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {showImport && (
        <div className="space-y-2 rounded-md border border-border p-3">
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
            <p className={`text-sm ${message.error ? "text-destructive" : "text-green-500"}`}>
              {message.text}
            </p>
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
        </div>
      )}
    </div>
  )
}
