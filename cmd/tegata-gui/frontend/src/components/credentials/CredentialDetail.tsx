import { useCallback, useEffect, useRef, useState } from "react"
import { Copy, Check, Loader2, CheckCircle, AlertTriangle, ShieldCheck } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { TOTPCountdown } from "@/components/shared/TOTPCountdown"
import { App } from "@/lib/wails"
import { formatError, hashString } from "@/lib/utils"
import type { Credential, TOTPResult, AuditVerifyResult } from "@/lib/types"

interface CredentialDetailProps {
  credential: Credential | null
  onRemove: (id: string) => void
  auditEnabled: boolean
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function formatCredentialType(type: string): string {
  switch (type) {
    case "totp":
      return "TOTP"
    case "hotp":
      return "HOTP"
    case "static":
      return "Static password"
    case "challenge-response":
      return "Challenge-response"
    default:
      return type
  }
}

export function CredentialDetail({ credential, onRemove, auditEnabled }: CredentialDetailProps) {
  const [lastUsed, setLastUsed] = useState<string | null>(() => {
    if (!credential) return null
    const stored = localStorage.getItem(`last-used-${credential.id}`)
    return stored || null
  })

  // Confirmation dialog for credential deletion
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteConfirmInput, setDeleteConfirmInput] = useState("")

  // Audit state
  const [auditEventCount, setAuditEventCount] = useState<number | null>(null)
  const [verifyResult, setVerifyResult] = useState<AuditVerifyResult | null>(null)
  const [verifying, setVerifying] = useState(false)

  // Right sidebar panel width — persisted to localStorage
  const [metaPanelSize, setMetaPanelSize] = useState<number>(() => {
    const savedSize = localStorage.getItem("credential-meta-panel-size")
    return savedSize ? parseInt(savedSize, 10) : 280
  })

  const dragHandleRef = useRef<HTMLDivElement>(null)
  const isResizingRef = useRef(false)
  const startPosRef = useRef(0)
  const startSizeRef = useRef(0)

  // Update displayed "Last used" and audit event count when credential changes
  useEffect(() => {
    if (!credential) {
      setLastUsed(null)
      setAuditEventCount(null)
      setVerifyResult(null)
      return
    }
    const stored = localStorage.getItem(`last-used-${credential.id}`)
    setLastUsed(stored || null)
    setVerifyResult(null)

    if (auditEnabled) {
      // Fetch event count for this credential
      hashString(credential.label)
        .then((labelHash) =>
          App.GetAuditHistory().then((records) => {
            const count = (records ?? []).filter((r) => r.label_hash === labelHash).length
            setAuditEventCount(count)
          }),
        )
        .catch(() => setAuditEventCount(null))

      // Auto-verify integrity on credential selection
      setVerifying(true)
      App.VerifyCredentialAuditLog(credential.label)
        .then((result) => setVerifyResult(result ?? null))
        .catch(() =>
          setVerifyResult({
            valid: false,
            event_count: 0,
            error_detail: "Verification failed. Check your connection to the audit server.",
          }),
        )
        .finally(() => setVerifying(false))
    } else {
      setAuditEventCount(null)
    }
  }, [credential?.id, auditEnabled])

  const refreshAuditEventCount = useCallback(() => {
    if (!credential || !auditEnabled) return
    hashString(credential.label)
      .then((labelHash) =>
        App.GetAuditHistory().then((records) => {
          const count = (records ?? []).filter((r) => r.label_hash === labelHash).length
          setAuditEventCount(count)
        }),
      )
      .catch(() => {})
  }, [credential, auditEnabled])

  const handleVerify = useCallback(async () => {
    if (!credential) return
    setVerifying(true)
    setVerifyResult(null)
    try {
      const result = await App.VerifyCredentialAuditLog(credential.label)
      setVerifyResult(result ?? null)
    } catch {
      setVerifyResult({ valid: false, event_count: 0, error_detail: "Verification failed. Check your connection to the audit server." })
    } finally {
      setVerifying(false)
    }
  }, [credential])

  // Track "Last used" based on explicit user actions (Copy button, Generate button, etc.)
  const recordLastUsed = useCallback(() => {
    if (!credential) return
    const now = new Date()
    const formatted = now.toLocaleDateString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    })
    setLastUsed(formatted)
    localStorage.setItem(`last-used-${credential.id}`, formatted)
    refreshAuditEventCount()
  }, [credential, refreshAuditEventCount])

  // Drag-to-resize handlers for right sidebar panel width
  const handleMouseDown = (e: React.MouseEvent) => {
    isResizingRef.current = true
    startPosRef.current = e.clientX
    startSizeRef.current = metaPanelSize
    document.addEventListener("mousemove", handleMouseMove)
    document.addEventListener("mouseup", handleMouseUp)
  }

  const handleMouseMove = (e: MouseEvent) => {
    if (!isResizingRef.current) return

    const delta = e.clientX - startPosRef.current
    const newSize = startSizeRef.current - delta

    // Clamp size between 160px and 480px
    const clampedSize = Math.max(160, Math.min(480, newSize))
    setMetaPanelSize(clampedSize)
  }

  const handleMouseUp = () => {
    isResizingRef.current = false
    document.removeEventListener("mousemove", handleMouseMove)
    document.removeEventListener("mouseup", handleMouseUp)
    localStorage.setItem("credential-meta-panel-size", String(metaPanelSize))
  }

  if (!credential) {
    return (
      <main className="flex flex-1 items-center justify-center bg-background">
        <p className="text-muted-foreground">Select a credential</p>
      </main>
    )
  }

  return (
    <main className="flex flex-1 flex-row overflow-hidden bg-background">
      {/* Main action area */}
      <div className="flex flex-1 flex-col overflow-y-auto p-6">
        <div className="mb-4">
          <h2 className="text-xl font-semibold">{credential.label}</h2>
          {credential.issuer && (
            <p className="text-lg text-primary font-medium mt-1">{credential.issuer}</p>
          )}
          {(credential.tags ?? []).length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1">
              {(credential.tags ?? []).map((tag) => (
                <Badge key={tag} variant="secondary">
                  {tag}
                </Badge>
              ))}
            </div>
          )}
        </div>

        <Separator />

        <div className="flex-1 mt-4">
          {credential.type === "totp" && <TOTPView key={credential.label} credential={credential} onUsed={recordLastUsed} />}
          {credential.type === "hotp" && <HOTPView credential={credential} onUsed={recordLastUsed} />}
          {credential.type === "static" && <StaticView credential={credential} onUsed={recordLastUsed} />}
          {credential.type === "challenge-response" && <ChallengeResponseView credential={credential} onUsed={recordLastUsed} />}
        </div>
      </div>

      {/* Drag handle */}
      <div
        ref={dragHandleRef}
        className="w-1 cursor-col-resize bg-border hover:bg-primary/20 transition-colors"
        onMouseDown={handleMouseDown}
      />

      {/* Meta panel */}
      <div
        className="border-l border-border overflow-y-auto flex flex-col"
        style={{ width: `${metaPanelSize}px` }}
      >
        <div className="flex flex-1 flex-col p-4">
          <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">Details</h3>

          <div className="space-y-3 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Type</span>
              <span className="font-medium">{formatCredentialType(credential.type)}</span>
            </div>

            {credential.type === "totp" && (
              <>
                {credential.algorithm && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Algorithm</span>
                    <span className="font-medium font-mono">{credential.algorithm}</span>
                  </div>
                )}
                {credential.digits > 0 && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Digits</span>
                    <span className="font-medium">{credential.digits}</span>
                  </div>
                )}
                {credential.period > 0 && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Period</span>
                    <span className="font-medium">{credential.period}s</span>
                  </div>
                )}
              </>
            )}

            {credential.type === "hotp" && (
              <>
                {credential.algorithm && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Algorithm</span>
                    <span className="font-medium font-mono">{credential.algorithm}</span>
                  </div>
                )}
                {credential.digits > 0 && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Digits</span>
                    <span className="font-medium">{credential.digits}</span>
                  </div>
                )}
              </>
            )}

            {credential.type === "challenge-response" && credential.algorithm && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Algorithm</span>
                <span className="font-medium font-mono">{credential.algorithm}</span>
              </div>
            )}

            {credential.created_at && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Created</span>
                <span className="font-medium text-xs">{formatDate(credential.created_at)}</span>
              </div>
            )}

            {lastUsed && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Last used</span>
                <span className="font-medium text-xs">{lastUsed}</span>
              </div>
            )}
            {!lastUsed && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Last used</span>
                <span className="text-xs text-muted-foreground italic">Never</span>
              </div>
            )}
          </div>
        </div>

        {auditEnabled && (
          <>
            <Separator />
            <div className="p-4 space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase">Audit</h3>

              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Recorded actions</span>
                <span className="font-medium">
                  {auditEventCount === null ? "—" : auditEventCount}
                </span>
              </div>

              {verifying && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  Verifying…
                </div>
              )}

              {!verifying && verifyResult && (
                verifyResult.valid ? (
                  <div className="flex items-center justify-center gap-2 rounded-md border border-green-200 bg-green-50 p-2 dark:border-green-800 dark:bg-green-950">
                    <CheckCircle className="h-4 w-4 shrink-0 text-green-600 dark:text-green-400" />
                    <p className="text-xs text-green-700 dark:text-green-300">
                      {verifyResult.event_count === 0
                        ? "No audit events to verify."
                        : "Integrity verified."}
                    </p>
                  </div>
                ) : (
                  <div className="flex items-start gap-2 rounded-md border border-red-200 bg-red-50 p-2 dark:border-red-800 dark:bg-red-950">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-red-600 dark:text-red-400" />
                    <p className="text-xs font-semibold text-red-700 dark:text-red-300">
                      Tamper detected — {verifyResult.error_detail}
                    </p>
                  </div>
                )
              )}

              <Button
                variant="outline"
                size="sm"
                className="w-full"
                onClick={handleVerify}
                disabled={verifying}
              >
                <ShieldCheck className="mr-2 h-3 w-3" />
                Re-verify integrity
              </Button>
            </div>
          </>
        )}

        <div className="flex justify-end px-4 pt-2 pb-4">
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setShowDeleteConfirm(true)}
          >
            Remove credential
          </Button>
        </div>
      </div>

      {/* Delete confirmation dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove credential?</DialogTitle>
            <DialogDescription>
              This action cannot be undone. Type <span className="font-mono font-semibold">DELETE</span> to confirm removal of "{credential?.label}".
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <Input
              placeholder='Type "DELETE" to confirm'
              value={deleteConfirmInput}
              onChange={(e) => setDeleteConfirmInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && deleteConfirmInput === "DELETE" && credential) {
                  onRemove(credential.id)
                  setShowDeleteConfirm(false)
                  setDeleteConfirmInput("")
                }
              }}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setShowDeleteConfirm(false)
                setDeleteConfirmInput("")
              }}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (credential) {
                  onRemove(credential.id)
                  setShowDeleteConfirm(false)
                  setDeleteConfirmInput("")
                }
              }}
              disabled={deleteConfirmInput !== "DELETE"}
            >
              Remove
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </main>
  )
}

function TOTPView({ credential, onUsed }: { credential: Credential; onUsed: () => void }) {
  const [totp, setTotp] = useState<TOTPResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const fetchCode = useCallback(() => {
    App.GenerateTOTP(credential.label)
      .then((result) => {
        setError(null)
        if (result) {
          setTotp(result)
        }
      })
      .catch((err) => {
        setError(formatError(err, "Failed to generate code"))
      })
  }, [credential.label])

  useEffect(() => {
    fetchCode()
  }, [fetchCode])

  if (error) return (
    <div className="space-y-2">
      <p className="text-sm text-destructive">{error}</p>
      <Button variant="outline" size="sm" onClick={fetchCode}>Retry</Button>
    </div>
  )
  if (!totp) return <Loader2 className="h-6 w-6 animate-spin text-primary" />

  return (
    <div className="space-y-4">
      <TOTPCountdown
        code={totp.code}
        remaining={totp.remaining}
        period={credential.period || 30}
        onExpired={() => {
          // Don't refresh the code on countdown expiration
          // The code is time-based and will still be valid
          // This prevents unnecessary audit events from being recorded
        }}
      />
      <CopyButton
        copied={copied}
        onCopy={() => {
          navigator.clipboard.writeText(totp.code)
          setCopied(true)
          onUsed()
          setTimeout(() => setCopied(false), 2000)
        }}
      />
    </div>
  )
}

function HOTPView({ credential, onUsed }: { credential: Credential; onUsed: () => void }) {
  const [code, setCode] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  function generate() {
    setLoading(true)
    setError(null)
    App.GenerateHOTP(credential.label)
      .then((result) => {
        setCode(result)
        onUsed()
      })
      .catch((err) => setError(formatError(err, "Failed to generate code")))
      .finally(() => setLoading(false))
  }

  return (
    <div className="space-y-4">
      {error && <p className="text-sm text-destructive">{error}</p>}
      {code && (
        <span className="font-mono text-3xl font-bold tracking-wider">{code}</span>
      )}
      <div className="flex gap-2">
        <Button onClick={generate} disabled={loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Generate code"}
        </Button>
        {code && (
          <CopyButton
            copied={copied}
            onCopy={() => {
              navigator.clipboard.writeText(code)
              setCopied(true)
              setTimeout(() => setCopied(false), 2000)
            }}
          />
        )}
      </div>
      <p className="text-xs text-muted-foreground">Counter: {credential.counter}</p>
    </div>
  )
}

function StaticView({ credential, onUsed }: { credential: Credential; onUsed: () => void }) {
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  const [error, setError] = useState<string | null>(null)

  function copyPassword() {
    setLoading(true)
    setError(null)
    App.GetStaticPassword(credential.label)
      .then(() => {
        setCopied(true)
        onUsed()
        setTimeout(() => setCopied(false), 3000)
      })
      .catch((err) => {
        setError(formatError(err, "Failed to copy password"))
      })
      .finally(() => setLoading(false))
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        The password will be copied to your clipboard and auto-cleared after 45 seconds.
      </p>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button onClick={copyPassword} disabled={loading}>
        {loading ? (
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        ) : copied ? (
          <Check className="mr-2 h-4 w-4 text-green-500" />
        ) : (
          <Copy className="mr-2 h-4 w-4" />
        )}
        {copied ? "Copied — auto-clears in 45s" : "Copy to clipboard"}
      </Button>
    </div>
  )
}

function ChallengeResponseView({ credential, onUsed }: { credential: Credential; onUsed: () => void }) {
  const [challenge, setChallenge] = useState("")
  const [response, setResponse] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  function sign() {
    if (!challenge) return
    setLoading(true)
    setError(null)
    App.SignChallenge(credential.label, challenge)
      .then((result) => {
        setResponse(result)
        onUsed()
      })
      .catch((err) => setError(formatError(err, "Signing failed")))
      .finally(() => setLoading(false))
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Enter a challenge string to compute an HMAC signature using this credential's secret key. The resulting hex-encoded signature can be used for authentication with services that support challenge-response verification.
      </p>
      <div className="flex gap-2">
        <Input
          placeholder="Enter challenge text..."
          value={challenge}
          onChange={(e) => setChallenge(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && sign()}
        />
        <Button onClick={sign} disabled={!challenge || loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Sign"}
        </Button>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {response && (
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Signature</p>
          <code className="block break-all rounded bg-muted p-3 font-mono text-sm">
            {response}
          </code>
          <CopyButton
            copied={copied}
            onCopy={() => {
              navigator.clipboard.writeText(response)
              setCopied(true)
              setTimeout(() => setCopied(false), 2000)
            }}
          />
        </div>
      )}
    </div>
  )
}

function CopyButton({
  copied,
  onCopy,
}: {
  copied: boolean
  onCopy: () => void
}) {
  return (
    <Button variant="outline" size="sm" onClick={onCopy}>
      {copied ? (
        <Check className="mr-1 h-3 w-3 text-green-500" />
      ) : (
        <Copy className="mr-1 h-3 w-3" />
      )}
      {copied ? "Copied" : "Copy"}
    </Button>
  )
}
