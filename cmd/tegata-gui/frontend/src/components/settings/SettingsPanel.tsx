import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Info, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { StrengthMeter } from "@/components/shared/StrengthMeter"
import { useTheme } from "@/hooks/useTheme"
import { App } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"
import { cn, formatError } from "@/lib/utils"
import { DISMISS_OPTIONS } from "@/lib/constants"
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime"
import { SUPPORTED_LANGUAGES } from "@/lib/i18n"

interface SettingsPanelProps {
  open: boolean
  onClose: () => void
  onCredentialsChanged: () => void
  updateInfo: UpdateInfo | null
  onUpdateFound: (info: UpdateInfo | null) => void
}

export function SettingsPanel({ open, onClose, onCredentialsChanged, updateInfo, onUpdateFound }: SettingsPanelProps) {
  const { t, i18n: i18nInstance } = useTranslation()
  const { theme, setTheme } = useTheme()
  const currentLanguage = i18nInstance.language

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
  const [clipboardTimeout, setClipboardTimeout] = useState(45)
  const [auditConfigured, setAuditConfigured] = useState(false)
  const [autoStart, setAutoStart] = useState(false)
  const [appVersion, setAppVersion] = useState("")
  const [showTooltip, setShowTooltip] = useState(false)
  const [tooltipPos, setTooltipPos] = useState({ top: 0, left: 0 })
  const infoRef = useRef<HTMLDivElement>(null)

  const [updateChecking, setUpdateChecking] = useState(false)
  const [updateCheckDone, setUpdateCheckDone] = useState(false)
  const [updateCheckError, setUpdateCheckError] = useState("")

  useEffect(() => {
    if (!open) {
      setUpdateChecking(false)
      setUpdateCheckDone(false)
      setUpdateCheckError("")
    }
  }, [open])

  useEffect(() => {
    if (open) {
      App.GetIdleTimeoutSeconds()
        .then(setIdleTimeout)
        .catch(() => {})
      App.GetClipboardTimeoutSeconds()
        .then(setClipboardTimeout)
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

  async function handleClipboardTimeoutChange(seconds: number) {
    setClipboardTimeout(seconds)
    try {
      await App.SetClipboardTimeoutSeconds(seconds)
    } catch {
      // Revert on failure
      const current = await App.GetClipboardTimeoutSeconds()
      setClipboardTimeout(current)
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

  async function handleCheckForUpdates() {
    setUpdateChecking(true)
    setUpdateCheckDone(false)
    setUpdateCheckError("")
    try {
      const info = await App.CheckForUpdateManual()
      onUpdateFound(info)
      setUpdateCheckDone(true)
    } catch (err) {
      setUpdateCheckError(formatError(err, t("gui.settings.checkFailed")))
    } finally {
      setUpdateChecking(false)
    }
  }

  async function handleDismissUpdate(option: string) {
    if (!updateInfo) return
    try {
      await App.DismissUpdate(updateInfo.version, option)
      onUpdateFound(null)
    } catch {
      // Non-critical — ignore failures saving dismissal prefs.
    }
  }

  async function handleLanguageChange(lang: string) {
    try {
      await App.SetLanguage(lang)
    } catch {
      // Non-critical — still update the UI even if saving fails
    }
    // i18nInstance.changeLanguage() comes from the shim's useTranslation()
    // and updates the context state, triggering a full re-render.
    await i18nInstance.changeLanguage(lang)
  }

  if (!open) return null

  async function handleChangePassphrase() {
    setPassError("")
    if (newPass.length < 8) {
      setPassError(t("gui.settings.errorPassTooShort"))
      return
    }
    if (newPass !== confirmPass) {
      setPassError(t("gui.settings.errorPassNoMatch"))
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
      setPassError(formatError(err, t("gui.settings.errorChangeFailed")))
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

  const themeLabels: Record<string, string> = {
    system: t("gui.settings.themeSystem"),
    light: t("gui.settings.themeLight"),
    dark: t("gui.settings.themeDark"),
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg bg-card shadow-lg flex flex-col max-h-[80vh]">
        <div className="flex items-center justify-between border-b p-3">
          <h2 className="text-lg font-semibold">{t("gui.settings.title")}</h2>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="overflow-y-auto p-6">

        {/* Language */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionLanguage")}</h3>
          <div className="flex gap-1">
            {SUPPORTED_LANGUAGES.map((lang) => (
              <Button
                key={lang}
                variant={currentLanguage === lang ? "default" : "outline"}
                size="sm"
                onClick={() => handleLanguageChange(lang)}
                className={cn("flex-1", currentLanguage === lang && "pointer-events-none")}
              >
                {lang === "en-us" ? t("gui.settings.languageEnUs") : t("gui.settings.languageJaJp")}
              </Button>
            ))}
          </div>
        </section>

        <Separator className="my-4" />

        {/* Theme */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionTheme")}</h3>
          <div className="flex gap-1">
            {(["system", "light", "dark"] as const).map((themeOption) => (
              <Button
                key={themeOption}
                variant={theme === themeOption ? "default" : "outline"}
                size="sm"
                onClick={() => setTheme(themeOption)}
                className={cn("flex-1", theme === themeOption && "pointer-events-none")}
              >
                {themeLabels[themeOption]}
              </Button>
            ))}
          </div>
        </section>

        <Separator className="my-4" />

        {/* Auto-lock */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionAutoLock")}</h3>
          <select
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={idleTimeout}
            onChange={(e) => handleIdleTimeoutChange(Number(e.target.value))}
          >
            {![60, 120, 300, 600, 900, 1800, 0].includes(idleTimeout) && (
              <option value={idleTimeout}>{t("gui.settings.autoLockCustom", { seconds: idleTimeout })}</option>
            )}
            <option value={60}>{t("gui.settings.autoLock1min")}</option>
            <option value={120}>{t("gui.settings.autoLock2min")}</option>
            <option value={300}>{t("gui.settings.autoLock5min")}</option>
            <option value={600}>{t("gui.settings.autoLock10min")}</option>
            <option value={900}>{t("gui.settings.autoLock15min")}</option>
            <option value={1800}>{t("gui.settings.autoLock30min")}</option>
            <option value={0}>{t("gui.settings.autoLockNever")}</option>
          </select>
          <p className="text-xs text-muted-foreground">
            {t("gui.settings.autoLockDescription")}
          </p>
        </section>

        <Separator className="my-4" />

        {/* Clipboard */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionClipboard")}</h3>
          <select
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={clipboardTimeout}
            onChange={(e) => handleClipboardTimeoutChange(Number(e.target.value))}
          >
            {![15, 30, 45, 60, 120, 0].includes(clipboardTimeout) && (
              <option value={clipboardTimeout}>{t("gui.settings.clipboardCustom", { seconds: clipboardTimeout })}</option>
            )}
            <option value={15}>{t("gui.settings.clipboard15s")}</option>
            <option value={30}>{t("gui.settings.clipboard30s")}</option>
            <option value={45}>{t("gui.settings.clipboard45s")}</option>
            <option value={60}>{t("gui.settings.clipboard60s")}</option>
            <option value={120}>{t("gui.settings.clipboard120s")}</option>
            <option value={0}>{t("gui.settings.clipboardNever")}</option>
          </select>
          <p className="text-xs text-muted-foreground">
            {t("gui.settings.clipboardDescription")}
          </p>
        </section>

        <Separator className="my-4" />

        {/* Vault */}
        <section className="space-y-3">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionVault")}</h3>

          {!showPassChange ? (
            <Button variant="outline" size="sm" onClick={() => setShowPassChange(true)}>
              {t("gui.settings.changePassphrase")}
            </Button>
          ) : (
            <div className="space-y-2 rounded-md border border-border p-3">
              <Input
                type="password"
                placeholder={t("gui.settings.currentPassphrasePlaceholder")}
                value={currentPass}
                onChange={(e) => setCurrentPass(e.target.value)}
              />
              <Input
                type="password"
                placeholder={t("gui.settings.newPassphrasePlaceholder")}
                value={newPass}
                onChange={(e) => setNewPass(e.target.value)}
              />
              {newPass && <StrengthMeter passphrase={newPass} />}
              <Input
                type="password"
                placeholder={t("gui.settings.confirmNewPassphrasePlaceholder")}
                value={confirmPass}
                onChange={(e) => setConfirmPass(e.target.value)}
              />
              {passError && <p className="text-sm text-destructive">{passError}</p>}
              {passSuccess && <p className="text-sm text-green-500">{t("gui.settings.passphraseChanged")}</p>}
              <div className="flex gap-2">
                <Button size="sm" onClick={handleChangePassphrase}>{t("gui.common.save")}</Button>
                <Button size="sm" variant="outline" onClick={() => {
                  setShowPassChange(false)
                  setCurrentPass("")
                  setNewPass("")
                  setConfirmPass("")
                  setPassError("")
                }}>
                  {t("gui.common.cancel")}
                </Button>
              </div>
            </div>
          )}

          {!showRecovery ? (
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" onClick={() => setShowRecovery(true)}>
                {t("gui.settings.verifyRecovery")}
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
                    {t("gui.settings.recoveryTooltip")}
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="space-y-2 rounded-md border border-border p-3">
              <Input
                placeholder={t("gui.settings.recoveryPlaceholder")}
                value={recoveryKey}
                onChange={(e) => { setRecoveryKey(e.target.value); setRecoveryResult(null) }}
                className="font-mono"
              />
              {recoveryResult === true && <p className="text-sm text-green-500">{t("gui.settings.recoveryValid")}</p>}
              {recoveryResult === false && (
                <div className="space-y-2 rounded-md bg-destructive/10 p-3">
                  <p className="text-sm font-medium text-destructive">{t("gui.settings.recoveryInvalid")}</p>
                  <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
                    <li>{t("gui.settings.recoveryHint1")}</li>
                    <li>{t("gui.settings.recoveryHint2")}</li>
                    <li>{t("gui.settings.recoveryHint3")}</li>
                    <li>{t("gui.settings.recoveryHint4")}</li>
                  </ul>
                </div>
              )}
              <div className="flex gap-2">
                <Button size="sm" onClick={handleVerifyRecovery}>{t("gui.common.verify")}</Button>
                <Button size="sm" variant="outline" onClick={() => {
                  setShowRecovery(false)
                  setRecoveryKey("")
                  setRecoveryResult(null)
                }}>
                  {t("gui.common.cancel")}
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
              <h3 className="text-sm font-medium">{t("gui.settings.sectionAudit")}</h3>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={autoStart}
                  onChange={(e) => handleAutoStartChange(e.target.checked)}
                  className="rounded border-input"
                />
                {t("gui.settings.autoStartLedger")}
              </label>
              <p className="text-xs text-muted-foreground">
                {t("gui.settings.autoStartDescription")}
              </p>
            </section>
          </>
        )}

        <Separator className="my-4" />

        {/* Updates */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionUpdates")}</h3>
          {updateInfo ? (
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                {t("gui.settings.updateAvailable", { version: updateInfo.version })}
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => BrowserOpenURL(updateInfo.url)}
              >
                {t("gui.settings.download")}
              </Button>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("gui.settings.remindMe")}</p>
                <div className="flex flex-wrap gap-2">
                  <Button variant="outline" size="sm" onClick={() => handleDismissUpdate(DISMISS_OPTIONS.tomorrow)}>
                    {t("gui.settings.tomorrow")}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => handleDismissUpdate(DISMISS_OPTIONS.oneMonth)}>
                    {t("gui.settings.inOneMonth")}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => handleDismissUpdate(DISMISS_OPTIONS.nextRelease)}>
                    {t("gui.settings.notUntilNextRelease")}
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              {updateCheckDone && !updateCheckError && (
                <p className="text-sm text-green-500">{t("gui.settings.upToDate")}</p>
              )}
              {updateCheckError && (
                <p className="text-sm text-destructive">{updateCheckError}</p>
              )}
              <Button
                variant="outline"
                size="sm"
                onClick={handleCheckForUpdates}
                disabled={updateChecking || updateCheckDone}
              >
                {updateChecking ? t("gui.settings.checking") : t("gui.settings.checkForUpdates")}
              </Button>
            </div>
          )}
        </section>

        <Separator className="my-4" />

        {/* About */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionAbout")}</h3>
          <p className="text-xs text-muted-foreground">{t("gui.settings.aboutDescription")}</p>
          {appVersion && <p className="text-xs text-muted-foreground">{t("gui.settings.aboutVersion", { version: appVersion })}</p>}
          <p className="text-xs text-muted-foreground">{t("gui.settings.aboutLicense")}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => BrowserOpenURL(t("gui.settings.privacyAndDisclaimerUrl"))}
          >
            {t("gui.settings.privacyAndDisclaimer")}
          </Button>
        </section>

        <Separator className="my-4" />

        {/* Support */}
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{t("gui.settings.sectionSupport")}</h3>
          <p className="text-xs text-muted-foreground">{t("gui.settings.supportDescription")}</p>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => BrowserOpenURL(t("gui.settings.supportGitHubSponsorsUrl"))}
            >
              {t("gui.settings.supportGitHubSponsors")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => BrowserOpenURL(t("gui.settings.supportKoFiUrl"))}
            >
              {t("gui.settings.supportKoFi")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => BrowserOpenURL(t("gui.settings.supportPayPalUrl"))}
            >
              {t("gui.settings.supportPayPal")}
            </Button>
          </div>
        </section>
        </div>
      </div>
    </div>
  )
}

function ExportImport({ onImported }: { onImported: () => void }) {
  const { t } = useTranslation()
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
      setMessage({ text: t("gui.settings.errorExportTooShort"), error: true })
      return
    }
    if (exportPass !== exportConfirm) {
      setMessage({ text: t("gui.settings.errorExportNoMatch"), error: true })
      return
    }
    setLoading(true)
    setMessage(null)
    try {
      const path = await App.ExportVaultToFile(exportPass)
      if (path) {
        setMessage({ text: t("gui.settings.exportedTo", { path }), error: false })
        setExportPass("")
        setExportConfirm("")
      }
    } catch (err) {
      setMessage({ text: formatError(err, t("gui.settings.exportFailed")), error: true })
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
      setMessage({ text: formatError(err, t("gui.settings.selectFileFailed")), error: true })
    }
  }

  async function handleImport() {
    if (!importPass || !importFile) return
    setLoading(true)
    setMessage(null)
    try {
      const result = await App.ImportVaultFromFile(importFile, importPass)
      if (result) {
        setMessage({ text: t("gui.settings.importResult", { imported: result.imported, skipped: result.skipped }), error: false })
        setImportPass("")
        setImportFile(null)
        onImported()
      }
    } catch (err) {
      setMessage({ text: formatError(err, t("gui.settings.importFailed")), error: true })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-2 pt-2">
      {!showExport && !showImport && (
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => { setShowExport(true); setMessage(null) }}>
            {t("gui.settings.exportCredentials")}
          </Button>
          <Button variant="outline" size="sm" onClick={() => { setShowImport(true); setMessage(null) }}>
            {t("gui.settings.importCredentials")}
          </Button>
        </div>
      )}

      {showExport && (
        <div className="space-y-2 rounded-md border border-border p-3">
          {message && !message.error ? (
            <>
              <p className="text-sm text-green-500">{message.text}</p>
              <Button size="sm" variant="outline" onClick={() => { setShowExport(false); setExportPass(""); setExportConfirm(""); setMessage(null) }}>
                {t("gui.common.done")}
              </Button>
            </>
          ) : (
            <>
              <p className="text-xs text-muted-foreground">
                {t("gui.settings.exportDescription")}
              </p>
              <Input
                type="password"
                placeholder={t("gui.settings.exportPassphrasePlaceholder")}
                value={exportPass}
                onChange={(e) => { setExportPass(e.target.value); setMessage(null) }}
              />
              {exportPass.length > 0 && <StrengthMeter passphrase={exportPass} />}
              <Input
                type="password"
                placeholder={t("gui.settings.exportConfirmPlaceholder")}
                value={exportConfirm}
                onChange={(e) => { setExportConfirm(e.target.value); setMessage(null) }}
              />
              {message && (
                <p className="text-sm text-destructive">{message.text}</p>
              )}
              <div className="flex gap-2">
                <Button size="sm" onClick={handleExport} disabled={!exportPass || !exportConfirm || loading}>
                  {loading ? t("gui.settings.exporting") : t("gui.settings.exportToFile")}
                </Button>
                <Button size="sm" variant="outline" onClick={() => { setShowExport(false); setExportPass(""); setExportConfirm(""); setMessage(null) }} disabled={loading}>
                  {t("gui.common.cancel")}
                </Button>
              </div>
            </>
          )}
        </div>
      )}

      {showImport && (
        <div className="space-y-2 rounded-md border border-border p-3">
          {message && !message.error ? (
            <>
              <p className="text-sm text-green-500">{message.text}</p>
              <Button size="sm" variant="outline" onClick={() => { setShowImport(false); setImportPass(""); setImportFile(null); setMessage(null) }}>
                {t("gui.common.done")}
              </Button>
            </>
          ) : (
            <>
              {!importFile ? (
                <>
                  <p className="text-xs text-muted-foreground">
                    {t("gui.settings.importDescription")}
                  </p>
                  <Button size="sm" onClick={handlePickFile}>
                    {t("gui.settings.chooseFile")}
                  </Button>
                </>
              ) : (
                <>
                  <p className="text-xs text-muted-foreground">
                    {t("gui.settings.importFile", { name: importFile.split(/[/\\]/).pop() })}
                  </p>
                  <Input
                    type="password"
                    placeholder={t("gui.settings.importPassphrasePlaceholder")}
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
                    {loading ? t("gui.settings.importing") : t("gui.settings.import")}
                  </Button>
                )}
                <Button size="sm" variant="outline" onClick={() => { setShowImport(false); setImportPass(""); setImportFile(null); setMessage(null) }}>
                  {t("gui.common.cancel")}
                </Button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}
