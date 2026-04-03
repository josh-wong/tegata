import { useEffect, useState, useCallback } from "react"
import { AlertTriangle, CheckCircle, Shield, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { App } from "@/lib/wails"
import type { AuditHistoryRecord, AuditVerifyResult } from "@/lib/types"
import { cn, formatError } from "@/lib/utils"
import { StopWipeConfirmDialog } from "./StopWipeConfirmDialog"

async function hashString(s: string): Promise<string> {
  const data = new TextEncoder().encode(s)
  const digest = await crypto.subtle.digest("SHA-256", data)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
}

const operationLabels: Record<string, string> = {
  totp: "TOTP",
  hotp: "HOTP",
  static: "Static password",
  "challenge-response": "Challenge-response",
}

async function buildLabelMap(): Promise<Record<string, string>> {
  try {
    const creds = await App.ListCredentials()
    const map: Record<string, string> = {}
    for (const cred of creds || []) {
      const hash = await hashString(cred.label)
      map[hash] = cred.label
    }
    return map
  } catch {
    return {}
  }
}

interface AuditPanelProps {
  open: boolean
  onClose: () => void
}

export function AuditPanel({ open, onClose }: AuditPanelProps) {
  const [history, setHistory] = useState<AuditHistoryRecord[]>([])
  const [verifyResult, setVerifyResult] = useState<AuditVerifyResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [labelMap, setLabelMap] = useState<Record<string, string>>({})
  const [setupSteps, setSetupSteps] = useState<string[]>([])
  const [setupStatus, setSetupStatus] = useState<"idle" | "in-progress" | "complete" | "error">("idle")
  const [dockerComposePath, setDockerComposePath] = useState("")
  const [wipeDialogOpen, setWipeDialogOpen] = useState(false)

  useEffect(() => {
    if (open) {
      setError("")
      setVerifyResult(null)
      buildLabelMap().then(setLabelMap)
      // Check if Docker audit setup has been run by calling GetAuditDockerPath.
      // Returns empty string when setup has not been run.
      App.GetAuditDockerPath().then((path) => setDockerComposePath(path ?? "")).catch(() => setDockerComposePath(""))
    }
  }, [open])

  const resolveLabel = useCallback(
    (hash: string) => labelMap[hash] ?? "(deleted)",
    [labelMap],
  )

  async function handleStartAuditServer() {
    setSetupStatus("in-progress")
    setSetupSteps([])
    setError("")
    try {
      const result = await App.StartAuditServer()
      setSetupSteps(result?.steps ?? [])
      setSetupStatus("complete")
      // Re-check docker path so Stop button appears.
      const path = await App.GetAuditDockerPath()
      setDockerComposePath(path ?? "")
    } catch (err) {
      setSetupStatus("error")
      setError(formatError(err, "Failed to start ledger server"))
    }
  }

  async function handleStopAuditServer() {
    setLoading(true)
    setError("")
    try {
      await App.StopAuditServer(false)
      setDockerComposePath("")
      setSetupStatus("idle")
    } catch (err) {
      setError(formatError(err, "Failed to stop ledger server"))
    } finally {
      setLoading(false)
    }
  }

  async function handleFetchHistory() {
    setLoading(true)
    setError("")
    try {
      const records = await App.GetAuditHistory()
      setHistory(records || [])
    } catch (err) {
      setError(formatError(err, "Failed to fetch audit data"))
    } finally {
      setLoading(false)
    }
  }

  async function handleVerify() {
    setLoading(true)
    setError("")
    setVerifyResult(null)
    try {
      const result = await App.VerifyAuditLog()
      setVerifyResult(result)
    } catch (err) {
      setError(formatError(err, "Failed to verify audit log"))
    } finally {
      setLoading(false)
    }
  }

  if (!open) return null

  return (
    <>
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
        <div className="bg-background border rounded-lg shadow-lg w-full max-w-lg max-h-[80vh] overflow-hidden flex flex-col">
          <div className="flex items-center justify-between p-4 border-b">
            <div className="flex items-center gap-2">
              <Shield className="h-5 w-5" />
              <h2 className="text-lg font-semibold">Audit</h2>
            </div>
            <Button variant="ghost" size="icon" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
          </div>

          <div className="p-4 space-y-4 overflow-y-auto flex-1">
            {!dockerComposePath && setupStatus === "idle" && history.length === 0 && !verifyResult && (
              <p className="text-sm text-muted-foreground">
                Audit logging is not configured. Start the ledger server to enable tamper-evident logging.
              </p>
            )}

            <div className="flex flex-wrap gap-2">
              {dockerComposePath ? (
                <>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleStopAuditServer}
                    disabled={loading || setupStatus === "in-progress"}
                  >
                    Stop ledger server
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setWipeDialogOpen(true)}
                    disabled={loading || setupStatus === "in-progress"}
                  >
                    Delete history...
                  </Button>
                </>
              ) : (
                <Button
                  variant="default"
                  size="sm"
                  onClick={handleStartAuditServer}
                  disabled={loading || setupStatus === "in-progress"}
                >
                  Start ledger server
                </Button>
              )}
              <Button
                onClick={handleFetchHistory}
                disabled={loading || setupStatus === "in-progress"}
                variant="outline"
                size="sm"
              >
                View history
              </Button>
              <Button
                onClick={handleVerify}
                disabled={loading || setupStatus === "in-progress"}
                variant="outline"
                size="sm"
              >
                Verify integrity
              </Button>
            </div>

            {(setupSteps.length > 0 || setupStatus === "in-progress") && (
              <div className="space-y-1 text-sm text-muted-foreground mt-4">
                {setupSteps.map((step, i) => (
                  <div key={i}>{step}</div>
                ))}
                {setupStatus === "complete" && (
                  <div className="text-green-600 dark:text-green-400">
                    Ledger server started. Audit logging is now active.
                  </div>
                )}
                {setupStatus === "error" && error && (
                  <div className="text-destructive">{error}</div>
                )}
                {setupStatus === "in-progress" && (
                  <div>Starting ledger server...</div>
                )}
              </div>
            )}

            {error && setupStatus !== "error" && (
              <p className="text-sm text-destructive">{error}</p>
            )}

            {verifyResult && (
              <div className={cn(
                "p-3 rounded-md border",
                verifyResult.valid
                  ? "bg-green-50 border-green-200 dark:bg-green-950 dark:border-green-800"
                  : "bg-red-50 border-red-200 dark:bg-red-950 dark:border-red-800"
              )}>
                {verifyResult.valid ? (
                  verifyResult.event_count === 0 ? (
                    <p className="text-sm">No audit events found. Nothing to verify.</p>
                  ) : (
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                      <p className="text-sm text-green-700 dark:text-green-300">
                        Audit log integrity verified. {verifyResult.event_count} events checked.
                      </p>
                    </div>
                  )
                ) : (
                  <div className="flex items-center gap-2">
                    <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400" />
                    <p className="text-sm font-semibold text-red-700 dark:text-red-300">
                      TAMPER DETECTED: {verifyResult.error_detail}
                    </p>
                  </div>
                )}
              </div>
            )}

            {history.length === 0 && !loading && !error && !verifyResult && (
              <p className="text-sm text-muted-foreground">No audit events found. Generate a code or perform a credential action to create events.</p>
            )}

            {history.length > 0 && (
              <>
                <Separator />
                <div className="space-y-1">
                  <p className="text-sm font-medium">{history.length} events</p>
                  <div className="border rounded-md overflow-x-auto">
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b bg-muted/50">
                          <th className="text-left p-2">Operation</th>
                          <th className="text-left p-2">Label</th>
                          <th className="text-left p-2">Timestamp</th>
                          <th className="text-left p-2">Hash</th>
                        </tr>
                      </thead>
                      <tbody>
                        {history.map((record, i) => (
                          <tr key={i} className="border-b last:border-0">
                            <td className="p-2">{operationLabels[record.operation] ?? record.operation}</td>
                            <td className="p-2">{resolveLabel(record.label_hash)}</td>
                            <td className="p-2 text-muted-foreground">
                              {record.timestamp ? new Date(record.timestamp * 1000).toLocaleString() : "\u2014"}
                            </td>
                            <td className="p-2 font-mono truncate max-w-[200px]">{record.hash_value}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      <StopWipeConfirmDialog
        open={wipeDialogOpen}
        onClose={() => setWipeDialogOpen(false)}
        onWipeComplete={() => {
          setDockerComposePath("")
          setSetupStatus("idle")
        }}
      />
    </>
  )
}
