import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { App } from "@/lib/wails"
import { cn } from "@/lib/utils"
import type { CredentialType } from "@/lib/types"

interface AddCredentialDialogProps {
  open: boolean
  onClose: () => void
  onAdded: () => void
}

export function AddCredentialDialog({ open, onClose, onAdded }: AddCredentialDialogProps) {
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

  // URI state
  const [uri, setUri] = useState("")

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
      setError("Label and secret are required")
      return
    }
    if ((credType === "totp" || credType === "hotp") && !isValidBase32(secret)) {
      setError("Secret contains invalid characters — TOTP and HOTP secrets use A-Z and 2-7 only (0, 1, and 8 are also accepted as lookalikes)")
      return
    }
    setLoading(true)
    setError("")
    try {
      const tagList = tags
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean)
      await App.AddCredential(label, issuer, credType, secret, algorithm, digits, period, tagList)
      reset()
      onAdded()
      onClose()
    } catch (err) {
      setError(typeof err === "string" ? err : err instanceof Error ? err.message : "Failed to add credential")
    } finally {
      setLoading(false)
    }
  }

  async function handleURISubmit(e: FormEvent) {
    e.preventDefault()
    if (!uri) {
      setError("URI is required")
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
      setError(typeof err === "string" ? err : err instanceof Error ? err.message : "Failed to add credential")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg bg-card p-6 shadow-lg">
        <h2 className="mb-4 text-lg font-semibold">Add credential</h2>

        <Tabs value={tab} onValueChange={setTab}>
          <TabsList className="mb-4 w-full">
            <TabsTrigger value="manual" className="flex-1">Manual entry</TabsTrigger>
            <TabsTrigger value="uri" className="flex-1">Paste URI</TabsTrigger>
          </TabsList>

          <TabsContent value="manual">
            <form onSubmit={handleManualSubmit} className="space-y-3">
              <Input
                placeholder="Label (required)"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                autoFocus
              />
              <Input
                placeholder="Issuer (optional)"
                value={issuer}
                onChange={(e) => setIssuer(e.target.value)}
              />
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={credType}
                onChange={(e) => setCredType(e.target.value as CredentialType)}
              >
                <option value="totp">TOTP</option>
                <option value="hotp">HOTP</option>
                <option value="static">Static password</option>
                <option value="challenge-response">Challenge-response</option>
              </select>
              <div className="space-y-1.5">
                <Input
                  type="password"
                  placeholder={
                    credType === "static" ? "Password (required)" :
                    credType === "challenge-response" ? "Shared secret key (required)" :
                    "Secret (required)"
                  }
                  value={secret}
                  onChange={(e) => setSecret(e.target.value)}
                />
                {credType === "static" && (
                  <p className="text-xs text-muted-foreground">
                    This is the password that will be stored and copied to your clipboard when you use this credential.
                  </p>
                )}
                {credType === "challenge-response" && (
                  <p className="text-xs text-muted-foreground">
                    The shared secret used to compute HMAC signatures. Can be plain text or a base32-encoded key.
                  </p>
                )}
              </div>
              {(credType === "totp" || credType === "hotp" || credType === "challenge-response") && (
                <div className="flex gap-2">
                  <select
                    className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={algorithm}
                    onChange={(e) => setAlgorithm(e.target.value)}
                  >
                    <option value="SHA1">SHA-1</option>
                    <option value="SHA256">SHA-256</option>
                    <option value="SHA512">SHA-512</option>
                  </select>
                  <select
                    className={cn("w-20 rounded-md border border-input bg-background px-3 py-2 text-sm", credType === "challenge-response" && "invisible")}
                    value={digits}
                    onChange={(e) => setDigits(Number(e.target.value))}
                  >
                    <option value={6}>6</option>
                    <option value={8}>8</option>
                  </select>
                  <Input
                    type="number"
                    className={cn("w-20", credType !== "totp" && "invisible")}
                    value={period}
                    onChange={(e) => setPeriod(Number(e.target.value))}
                    min={15}
                    max={120}
                  />
                </div>
              )}
              <Input
                placeholder="Tags (comma-separated)"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
              />

              {error && <p className="text-sm text-destructive">{error}</p>}

              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => { reset(); onClose() }}>
                  Cancel
                </Button>
                <Button type="submit" disabled={loading}>
                  {loading ? "Adding..." : "Add"}
                </Button>
              </div>
            </form>
          </TabsContent>

          <TabsContent value="uri">
            <form onSubmit={handleURISubmit} className="space-y-3">
              <textarea
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
                rows={4}
                placeholder="otpauth://totp/Example:user@example.com?secret=..."
                value={uri}
                onChange={(e) => setUri(e.target.value)}
                autoFocus
              />

              {error && <p className="text-sm text-destructive">{error}</p>}

              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => { reset(); onClose() }}>
                  Cancel
                </Button>
                <Button type="submit" disabled={loading || !uri}>
                  {loading ? "Adding..." : "Add"}
                </Button>
              </div>
            </form>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
