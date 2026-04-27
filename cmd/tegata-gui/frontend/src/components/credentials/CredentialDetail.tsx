import { useCallback, useEffect, useRef, useState } from "react"
import { Copy, Check, Loader2 } from "lucide-react"
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
import { formatError, formatTimestamp, hashString } from "@/lib/utils"
import type { Credential, TOTPResult } from "@/lib/types"

interface CredentialDetailProps {
  credential: Credential | null
  onRemove: (id: string) => void
  auditEnabled?: boolean
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

export function CredentialDetail({ credential, onRemove, auditEnabled = false }: CredentialDetailProps) {
  // Reset derived state when the selected credential changes, using React's
  // "setState during render" pattern to avoid the react-hooks/set-state-in-effect rule.
  // See: https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes
  const [lastUsed, setLastUsed] = useState<string | null>(null)
  const [auditEventCount, setAuditEventCount] = useState<number | null>(null)
  const [prevCredId, setPrevCredId] = useState<string | null>(credential?.id ?? null)
  if ((credential?.id ?? null) !== prevCredId) {
    setPrevCredId(credential?.id ?? null)
    setLastUsed(credential ? localStorage.getItem(`last-used-${credential.id}`) || null : null)
    setAuditEventCount(null)
  }

  // Confirmation dialog for credential deletion
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteConfirmInput, setDeleteConfirmInput] = useState("")

  // Shared fetch for audit event count — used on initial load and after user actions.
  const fetchAuditEventCount = useCallback(() => {
    if (!credential || !auditEnabled) return
    hashString(credential.label)
      .then((labelHash) =>
        App.GetAuditHistory().then((records) => {
          const count = (records ?? []).filter((r) => r.label_hash === labelHash).length
          setAuditEventCount(count)
        }),
      )
      .catch(() => setAuditEventCount(null))
  }, [credential, auditEnabled])

  // Fetch audit event count when the selected credential or audit state changes.
  useEffect(() => {
    fetchAuditEventCount()
  }, [fetchAuditEventCount])

  // Track "Last used" based on explicit user actions (Copy button, Generate button, etc.).
  // Writes to localStorage and updates state immediately.
  const recordLastUsed = useCallback(() => {
    if (!credential) return
    const formatted = formatTimestamp(new Date())
    localStorage.setItem(`last-used-${credential.id}`, formatted)
    setLastUsed(formatted)
    fetchAuditEventCount()
  }, [credential, fetchAuditEventCount])

  // Shared handler for the delete confirmation dialog's confirm action.
  const confirmDelete = useCallback(() => {
    localStorage.removeItem(`last-used-${credential?.id}`)
    onRemove(credential!.id)
    setShowDeleteConfirm(false)
    setDeleteConfirmInput("")
  }, [credential, onRemove])


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
          {credential.type === "totp" && <TOTPView key={credential.id} credential={credential} onUsed={recordLastUsed} />}
          {credential.type === "hotp" && <HOTPView key={credential.id} credential={credential} onUsed={recordLastUsed} />}
          {credential.type === "static" && <StaticView key={credential.id} credential={credential} onUsed={recordLastUsed} />}
          {credential.type === "challenge-response" && <ChallengeResponseView key={credential.id} credential={credential} onUsed={recordLastUsed} />}
        </div>
      </div>

      {/* Meta panel */}
      <div className="w-72 border-l border-border overflow-y-auto flex flex-col">
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
                <span className="font-medium text-xs">{formatTimestamp(credential.created_at)}</span>
              </div>
            )}

            {credential.modified_at && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Modified</span>
                <span className="font-medium text-xs">{formatTimestamp(credential.modified_at)}</span>
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
                  {auditEventCount === null ? (
                    <Loader2 className="h-4 w-4 animate-spin inline" />
                  ) : (
                    auditEventCount
                  )}
                </span>
              </div>
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
      <Dialog open={showDeleteConfirm} onOpenChange={(open) => { setShowDeleteConfirm(open); if (!open) setDeleteConfirmInput("") }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove credential?</DialogTitle>
            <DialogDescription>
              This action cannot be undone. Type <span className="font-mono font-semibold">REMOVE</span> to confirm removal of "{credential.label}".
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <Input
              placeholder='Type "REMOVE" to confirm'
              value={deleteConfirmInput}
              onChange={(e) => setDeleteConfirmInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && deleteConfirmInput === "REMOVE") {
                  confirmDelete()
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
              onClick={confirmDelete}
              disabled={deleteConfirmInput !== "REMOVE"}
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
        onExpired={fetchCode}
      />
      <CopyButton
        copied={copied}
        onCopy={() => {
          App.CopyToClipboard(totp.code).catch(() => {})
          App.RecordTOTPUsed(credential.label).catch(() => {})
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
  const [copied, setCopied] = useState(false)
  const inFlight = useRef(false)

  function generate() {
    if (inFlight.current) return
    inFlight.current = true
    setError(null)
    App.GenerateHOTP(credential.label)
      .then((result) => {
        setCode(result)
        onUsed()
      })
      .catch((err) => setError(formatError(err, "Failed to generate code")))
      .finally(() => { inFlight.current = false })
  }

  return (
    <div className="space-y-4">
      {error && <p className="text-sm text-destructive">{error}</p>}
      {code && (
        <span className="font-mono text-3xl font-bold tracking-wider">{code}</span>
      )}
      <div className="flex gap-2">
        <Button onClick={generate}>Generate code</Button>
        {code && (
          <CopyButton
            copied={copied}
            onCopy={() => {
              App.CopyToClipboard(code).catch(() => {})
              setCopied(true)
              setTimeout(() => setCopied(false), 2000)
            }}
          />
        )}
      </div>
    </div>
  )
}

function StaticView({ credential, onUsed }: { credential: Credential; onUsed: () => void }) {
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inFlight = useRef(false)

  function copyPassword() {
    if (inFlight.current) return
    inFlight.current = true
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
      .finally(() => { inFlight.current = false })
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        The password will be copied to your clipboard and cleared automatically after the configured timeout.
      </p>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button onClick={copyPassword}>
        {copied ? (
          <Check className="mr-2 h-4 w-4 text-green-500" />
        ) : (
          <Copy className="mr-2 h-4 w-4" />
        )}
        {copied ? "Copied — auto-clears shortly" : "Copy to clipboard"}
      </Button>
    </div>
  )
}

function ChallengeResponseView({ credential, onUsed }: { credential: Credential; onUsed: () => void }) {
  const [challenge, setChallenge] = useState("")
  const [response, setResponse] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const inFlight = useRef(false)

  function sign() {
    if (!challenge || inFlight.current) return
    inFlight.current = true
    setError(null)
    App.SignChallenge(credential.label, challenge)
      .then((result) => {
        setResponse(result)
        onUsed()
      })
      .catch((err) => setError(formatError(err, "Signing failed")))
      .finally(() => { inFlight.current = false })
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
        <Button onClick={sign} disabled={!challenge}>Sign</Button>
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
              App.CopyToClipboard(response).catch(() => {})
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
    <Button variant="outline" onClick={onCopy}>
      {copied ? (
        <Check className="mr-2 h-4 w-4 text-green-500" />
      ) : (
        <Copy className="mr-2 h-4 w-4" />
      )}
      {copied ? "Copied" : "Copy"}
    </Button>
  )
}
