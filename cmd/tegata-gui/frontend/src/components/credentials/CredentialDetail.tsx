import { useCallback, useEffect, useState } from "react"
import { Copy, Check, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { TOTPCountdown } from "@/components/shared/TOTPCountdown"
import { App } from "@/lib/wails"
import type { Credential, TOTPResult } from "@/lib/types"

interface CredentialDetailProps {
  credential: Credential | null
  onRemove: (id: string) => void
}

export function CredentialDetail({ credential, onRemove }: CredentialDetailProps) {
  if (!credential) {
    return (
      <main className="flex flex-1 items-center justify-center bg-background">
        <p className="text-muted-foreground">Select a credential</p>
      </main>
    )
  }

  return (
    <main className="flex flex-1 flex-col bg-background p-6">
      <div className="mb-4">
        <h2 className="text-xl font-semibold">{credential.label}</h2>
        {credential.issuer && (
          <p className="text-sm text-muted-foreground">{credential.issuer}</p>
        )}
        {(credential.tags ?? []).length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1">
            {(credential.tags ?? []).map((tag) => (
              <Badge key={tag} variant="secondary">{tag}</Badge>
            ))}
          </div>
        )}
      </div>

      <Separator />

      <div className="mt-4 flex-1">
        {credential.type === "totp" && <TOTPView credential={credential} />}
        {credential.type === "hotp" && <HOTPView credential={credential} />}
        {credential.type === "static" && <StaticView credential={credential} />}
        {credential.type === "challenge-response" && <ChallengeResponseView credential={credential} />}
      </div>

      <Separator className="my-4" />

      <div className="flex justify-end">
        <Button
          variant="destructive"
          size="sm"
          onClick={() => onRemove(credential.id)}
        >
          Remove credential
        </Button>
      </div>
    </main>
  )
}

function TOTPView({ credential }: { credential: Credential }) {
  const [totp, setTotp] = useState<TOTPResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const fetchCode = useCallback(() => {
    setError(null)
    App.GenerateTOTP(credential.label)
      .then((result) => {
        if (result) setTotp(result)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : String(err))
      })
  }, [credential.label])

  useEffect(() => {
    setTotp(null)
    fetchCode()
  }, [fetchCode])

  if (error) return <p className="text-sm text-destructive">{error}</p>
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
        text={totp.code}
        copied={copied}
        onCopy={() => {
          navigator.clipboard.writeText(totp.code)
          setCopied(true)
          setTimeout(() => setCopied(false), 2000)
        }}
      />
    </div>
  )
}

function HOTPView({ credential }: { credential: Credential }) {
  const [code, setCode] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  function generate() {
    setLoading(true)
    setError(null)
    App.GenerateHOTP(credential.label)
      .then(setCode)
      .catch((err) => setError(typeof err === "string" ? err : err instanceof Error ? err.message : "Failed to generate code"))
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
            text={code}
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

function StaticView({ credential }: { credential: Credential }) {
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  const [error, setError] = useState<string | null>(null)

  function copyPassword() {
    setLoading(true)
    setError(null)
    App.GetStaticPassword(credential.label)
      .then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 3000)
      })
      .catch((err) => {
        setError(typeof err === "string" ? err : err instanceof Error ? err.message : "Failed to copy password")
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

function ChallengeResponseView({ credential }: { credential: Credential }) {
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
      .then(setResponse)
      .catch((err) => setError(typeof err === "string" ? err : err instanceof Error ? err.message : "Signing failed"))
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
            text={response}
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
  text: _text,
  copied,
  onCopy,
}: {
  text: string
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
