import { useEffect, useId, useState } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { App } from "@/lib/wails"
import type { OwnerInfo } from "@/lib/types"
import { formatError } from "@/lib/utils"

// Must match ErrOwnerSaltChangedCode in cmd/tegata-gui/owner.go.
const SALT_CHANGED_CODE = "owner-salt-changed"
// Must match ErrOwnerConfigUnreadableCode in cmd/tegata-gui/owner.go.
const CONFIG_UNREADABLE_CODE = "owner-config-unreadable"
// Must match ErrOwnerInvalidCodePrefix in cmd/tegata-gui/owner.go. The full
// code is "owner-invalid:<field>:<reason>".
const INVALID_CODE_PREFIX = "owner-invalid:"
// Must match MaxOwnerIdentifierLen in internal/audit/owner.go.
const MAX_IDENTIFIER_LENGTH = 256

// OwnerIdentifiers lets the user set the employee ID and email that attribute
// audit events to them. Raw values are sent to the backend once, hashed there,
// and cleared from component state immediately after saving. The settings
// panel mounts it only while open, so closing the panel discards any input.
export function OwnerIdentifiers() {
  const { t } = useTranslation()
  const idFieldId = useId()
  const emailFieldId = useId()

  const [owner, setOwner] = useState<OwnerInfo | null>(null)
  const [editing, setEditing] = useState(false)
  const [employeeId, setEmployeeId] = useState("")
  const [email, setEmail] = useState("")
  const [clearEmployeeId, setClearEmployeeId] = useState(false)
  const [clearEmail, setClearEmail] = useState(false)
  const [error, setError] = useState("")
  const [loadFailed, setLoadFailed] = useState(false)
  // Result of the last save: "saved", "unchanged", or "" before any save.
  const [saveResult, setSaveResult] = useState<"" | "saved" | "unchanged">("")

  function resetForm() {
    setEditing(false)
    setEmployeeId("")
    setEmail("")
    setClearEmployeeId(false)
    setClearEmail(false)
    setError("")
  }

  useEffect(() => {
    App.GetOwner()
      .then((info) => { setOwner(info); setLoadFailed(false) })
      // Don't render "Not set" when the identifiers couldn't be read; they may
      // still be stored.
      .catch(() => { setOwner(null); setLoadFailed(true) })
  }, [])

  // saveErrorMessage maps the error codes that SetOwner returns to localized
  // messages and passes any other error through.
  function saveErrorMessage(msg: string): string {
    if (msg === SALT_CHANGED_CODE) return t("gui.settings.ownerSaltChanged")
    if (msg === CONFIG_UNREADABLE_CODE) return t("gui.settings.ownerConfigUnreadable")
    if (msg.startsWith(INVALID_CODE_PREFIX)) {
      const [field, reason] = msg.slice(INVALID_CODE_PREFIX.length).split(":")
      const key = `gui.settings.ownerInvalid.${field}.${reason}`
      const text = t(key, { max: MAX_IDENTIFIER_LENGTH })
      return text === key ? t("gui.settings.ownerSaveFailed") : text
    }
    return msg
  }

  async function handleSave() {
    setError("")
    setSaveResult("")
    try {
      const next = await App.SetOwner(employeeId.trim(), email.trim(), clearEmployeeId, clearEmail)
      setOwner(next)
      resetForm()
      setSaveResult(next.changed ? "saved" : "unchanged")
    } catch (err) {
      const msg = formatError(err, t("gui.settings.ownerSaveFailed"))
      setError(saveErrorMessage(msg))
      // Drop raw identifiers from state even when the save fails.
      setEmployeeId("")
      setEmail("")
    }
  }

  const current = (hash: string | undefined) =>
    hash ? t("gui.settings.ownerCurrent", { hash }) : t("gui.settings.ownerNotSet")

  return (
    <section className="space-y-2">
      <h3 className="text-sm font-medium">{t("gui.settings.sectionOwner")}</h3>
      <p className="text-xs text-muted-foreground">{t("gui.settings.ownerDescription")}</p>

      {loadFailed ? (
        <p className="text-sm text-destructive" role="alert">{t("gui.settings.ownerLoadFailed")}</p>
      ) : (
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
          <dt className="text-muted-foreground">{t("gui.settings.ownerEmployeeId")}</dt>
          <dd className="font-mono text-xs leading-5">{current(owner?.employeeIdHash)}</dd>
          <dt className="text-muted-foreground">{t("gui.settings.ownerEmail")}</dt>
          <dd className="font-mono text-xs leading-5">{current(owner?.emailHash)}</dd>
        </dl>
      )}
      {owner?.saltConfigured && (
        <p className="text-xs text-muted-foreground">{t("gui.settings.ownerSalted")}</p>
      )}
      {saveResult === "saved" && <p className="text-sm text-green-500" role="status">{t("gui.settings.ownerSaved")}</p>}
      {saveResult === "unchanged" && <p className="text-sm text-muted-foreground" role="status">{t("gui.settings.ownerUnchanged")}</p>}

      {loadFailed ? null : !editing ? (
        <Button variant="outline" size="sm" onClick={() => { setEditing(true); setSaveResult("") }}>
          {t("gui.settings.ownerEdit")}
        </Button>
      ) : (
        <div className="space-y-3 rounded-md border border-border p-3">
          <div className="space-y-1">
            <label htmlFor={idFieldId} className="text-sm">{t("gui.settings.ownerEmployeeId")}</label>
            <div className="flex items-center gap-2">
              <Input
                id={idFieldId}
                autoComplete="off"
                spellCheck={false}
                placeholder={t("gui.settings.ownerKeepPlaceholder")}
                value={employeeId}
                disabled={clearEmployeeId}
                onChange={(e) => setEmployeeId(e.target.value)}
              />
              {owner?.employeeIdHash && (
                <label className="flex shrink-0 items-center gap-1 text-xs">
                  <input
                    type="checkbox"
                    checked={clearEmployeeId}
                    aria-label={t("gui.settings.ownerClearEmployeeId")}
                    onChange={(e) => { setClearEmployeeId(e.target.checked); setEmployeeId("") }}
                    className="rounded border-input"
                  />
                  {t("gui.settings.ownerClear")}
                </label>
              )}
            </div>
          </div>
          <div className="space-y-1">
            <label htmlFor={emailFieldId} className="text-sm">{t("gui.settings.ownerEmail")}</label>
            <div className="flex items-center gap-2">
              <Input
                id={emailFieldId}
                type="email"
                autoComplete="off"
                spellCheck={false}
                placeholder={t("gui.settings.ownerKeepPlaceholder")}
                value={email}
                disabled={clearEmail}
                onChange={(e) => setEmail(e.target.value)}
              />
              {owner?.emailHash && (
                <label className="flex shrink-0 items-center gap-1 text-xs">
                  <input
                    type="checkbox"
                    checked={clearEmail}
                    aria-label={t("gui.settings.ownerClearEmail")}
                    onChange={(e) => { setClearEmail(e.target.checked); setEmail("") }}
                    className="rounded border-input"
                  />
                  {t("gui.settings.ownerClear")}
                </label>
              )}
            </div>
          </div>
          <p className="text-xs text-muted-foreground">{t("gui.settings.ownerPrefixTip")}</p>
          {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
          <div className="flex gap-2">
            <Button size="sm" onClick={handleSave}>{t("gui.common.save")}</Button>
            <Button size="sm" variant="outline" onClick={resetForm}>{t("gui.common.cancel")}</Button>
          </div>
        </div>
      )}
    </section>
  )
}
