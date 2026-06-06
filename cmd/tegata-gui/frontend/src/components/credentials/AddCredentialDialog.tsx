import { type FormEvent, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDown, ChevronRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { App } from "@/lib/wails"
import { formatError } from "@/lib/utils"
import type { CredentialType } from "@/lib/types"

interface AddCredentialDialogProps {
  open: boolean
  onClose: () => void
  onAdded: () => void
}

export function AddCredentialDialog({ open, onClose, onAdded }: AddCredentialDialogProps) {
  const { t } = useTranslation()
  const [tab, setTab] = useState("manual")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  // Manual entry state
  const [label, setLabel] = useState("")
  const [issuer, setIssuer] = useState("")
  const [credType, setCredType] = useState<CredentialType>("totp")
  const [secret, setSecret] = useState("")
  const [algorithm, setAlgorithm] = useState("SHA1")
  const [digits, setDigits] = useState(6)
  const [period, setPeriod] = useState(30)
  const [tags, setTags] = useState("")
  const [category, setCategory] = useState("")

  const [showAdvanced, setShowAdvanced] = useState(false)

  // URI state
  const [uri, setUri] = useState("")

  useEffect(() => {
    if (credType === "challenge-response") {
      setAlgorithm("SHA256")
      return
    }
    setAlgorithm("SHA1")
  }, [credType])

  if (!open) return null

  function reset() {
    setLabel("")
    setIssuer("")
    setCredType("totp")
    setSecret("")
    setAlgorithm("SHA1")
    setDigits(6)
    setPeriod(30)
    setTags("")
    setCategory("")
    setShowAdvanced(false)
    setUri("")
    setError("")
  }

  function isValidBase32(s: string): boolean {
    const cleaned = s
      .toUpperCase()
      .replace(/[\s\-=]/g, "")
      .replace(/0/g, "O")
      .replace(/1/g, "L")
      .replace(/8/g, "B")
    return /^[A-Z2-7]*$/.test(cleaned)
  }

  async function handleManualSubmit(e: FormEvent) {
    e.preventDefault()
    if (!label || !secret) {
      setError(t("gui.add.errorRequired"))
      return
    }
    if ((credType === "totp" || credType === "hotp") && !isValidBase32(secret)) {
      setError(t("gui.add.errorInvalidBase32"))
      return
    }
    setLoading(true)
    setError("")
    try {
      const effectiveAlgorithm =
        credType === "hotp"
          ? "SHA1"
          : algorithm

      const tagList = tags
        .split(",")
        .map((t) => t.trim().toLowerCase())
        .filter(Boolean)
      const normalizedCategory = category.trim().toLowerCase()
      await App.AddCredential(label, issuer, credType, secret, effectiveAlgorithm, digits, period, tagList, normalizedCategory)
      reset()
      onAdded()
      onClose()
    } catch (err) {
      setError(formatError(err, t("gui.add.errorAdd")))
    } finally {
      setLoading(false)
    }
  }

  async function handleURISubmit(e: FormEvent) {
    e.preventDefault()
    if (!uri) {
      setError(t("gui.add.errorUri"))
      return
    }
    setLoading(true)
    setError("")
    try {
      await App.AddCredentialFromURI(uri)
      reset()
      onAdded()
      onClose()
    } catch (err) {
      setError(formatError(err, t("gui.add.errorAdd")))
    } finally {
      setLoading(false)
    }
  }

  const secretPlaceholder =
    credType === "static" ? t("gui.add.secretPassword") :
    credType === "challenge-response" ? t("gui.add.secretSharedKey") :
    t("gui.add.secretDefault")

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg bg-card p-6 shadow-lg">
        <h2 className="mb-4 text-lg font-semibold">{t("gui.add.title")}</h2>

        <Tabs value={tab} onValueChange={setTab}>
          <TabsList className="mb-4 w-full">
            <TabsTrigger value="manual" className="flex-1">{t("gui.add.tabManual")}</TabsTrigger>
            <TabsTrigger value="uri" className="flex-1">{t("gui.add.tabUri")}</TabsTrigger>
          </TabsList>

          <TabsContent value="manual">
            <fieldset disabled={loading}>
            <form onSubmit={handleManualSubmit} className="space-y-3">
              <Input
                placeholder={t("gui.add.labelPlaceholder")}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                autoFocus
              />
              <Input
                placeholder={t("gui.add.issuerPlaceholder")}
                value={issuer}
                onChange={(e) => setIssuer(e.target.value)}
              />
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={credType}
                onChange={(e) => setCredType(e.target.value as CredentialType)}
              >
                <option value="totp">{t("gui.add.typeTotp")}</option>
                <option value="hotp">{t("gui.add.typeHotp")}</option>
                <option value="static">{t("gui.add.typeStatic")}</option>
                <option value="challenge-response">{t("gui.add.typeChallengeResponse")}</option>
              </select>
              <div className="space-y-1.5">
                <Input
                  type="password"
                  placeholder={secretPlaceholder}
                  value={secret}
                  onChange={(e) => setSecret(e.target.value)}
                />
                {credType === "static" && (
                  <p className="text-xs text-muted-foreground">
                    {t("gui.add.staticHint")}
                  </p>
                )}
                {credType === "challenge-response" && (
                  <p className="text-xs text-muted-foreground">
                    {t("gui.add.challengeHint")}
                  </p>
                )}
              </div>
              {(credType === "totp" || credType === "hotp" || credType === "challenge-response") && (
                <div className="space-y-2">
                  <button
                    type="button"
                    className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                    onClick={() => setShowAdvanced(!showAdvanced)}
                  >
                    {showAdvanced ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                    {t("gui.add.advancedOptions")}
                  </button>
                  {showAdvanced && (
                    <div className="space-y-2 rounded-md border border-border p-3">
                      {(credType === "totp" || credType === "challenge-response") && (
                        <div className="space-y-1">
                          <label className="text-xs text-muted-foreground">{t("gui.add.hashAlgorithm")}</label>
                          <select
                            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                            value={algorithm}
                            onChange={(e) => setAlgorithm(e.target.value)}
                          >
                            <option value="SHA1">{t("gui.add.sha1Default")}</option>
                            <option value="SHA256">{t("gui.add.sha256")}</option>
                            <option value="SHA512">{t("gui.add.sha512")}</option>
                          </select>
                          {credType === "totp" && (
                            <p className="text-xs text-muted-foreground">
                              {t("gui.add.totpAlgoHint")}
                            </p>
                          )}
                        </div>
                      )}
                      {credType !== "challenge-response" && (
                        <div className="space-y-1">
                          <label className="text-xs text-muted-foreground">{t("gui.add.codeLength")}</label>
                          <select
                            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                            value={digits}
                            onChange={(e) => setDigits(Number(e.target.value))}
                          >
                            <option value={6}>{t("gui.add.digits6")}</option>
                            <option value={8}>{t("gui.add.digits8")}</option>
                          </select>
                        </div>
                      )}
                      {credType === "totp" && (
                        <div className="space-y-1">
                          <label className="text-xs text-muted-foreground">{t("gui.add.refreshInterval")}</label>
                          <Input
                            type="number"
                            value={period}
                            onChange={(e) => setPeriod(Number(e.target.value))}
                            min={15}
                            max={120}
                          />
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}
              {credType === "hotp" && (
                <p className="text-xs text-muted-foreground">
                  {t("gui.add.hotpAlgoHint")}
                </p>
              )}
              <Input
                placeholder={t("gui.add.tagsPlaceholder")}
                value={tags}
                onChange={(e) => setTags(e.target.value)}
              />
              <Input
                placeholder={t("gui.add.categoryPlaceholder")}
                value={category}
                onChange={(e) => setCategory(e.target.value)}
              />

              {error && <p className="text-sm text-destructive">{error}</p>}

              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => { reset(); onClose() }}>
                  {t("gui.common.cancel")}
                </Button>
                <Button type="submit" disabled={loading}>
                  {loading ? t("gui.add.adding") : t("gui.add.add")}
                </Button>
              </div>
            </form>
            </fieldset>
          </TabsContent>

          <TabsContent value="uri">
            <fieldset disabled={loading}>
            <form onSubmit={handleURISubmit} className="space-y-3">
              <textarea
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
                rows={4}
                placeholder={t("gui.add.uriPlaceholder")}
                value={uri}
                onChange={(e) => setUri(e.target.value)}
                autoFocus
              />

              {error && <p className="text-sm text-destructive">{error}</p>}

              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => { reset(); onClose() }}>
                  {t("gui.common.cancel")}
                </Button>
                <Button type="submit" disabled={loading || !uri}>
                  {loading ? t("gui.add.adding") : t("gui.add.add")}
                </Button>
              </div>
            </form>
            </fieldset>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
