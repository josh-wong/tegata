import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { AlertTriangle, CheckCircle, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Shield, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { DatePicker } from "@/components/ui/date-picker"
import { App } from "@/lib/wails"
import type { AuditHistoryRecord, AuditVerifyResult } from "@/lib/types"
import { cn, formatError } from "@/lib/utils"

function formatFault(f: string): string {
  const idx = f.indexOf(": ")
  if (idx < 0) return f
  const id = f.slice(0, idx)
  const detail = f.slice(idx + 2)
  if (detail.startsWith("error: ")) {
    return `Verification error for record ${id}: ${detail.slice(7)}`
  }
  return `The ${detail} for record ${id}`
}

type SortCol = "operation" | "label" | "timestamp" | "hash"
type SortDir = "asc" | "desc"

const PAGE_SIZE = 10

// OPERATION_TYPE_LABELS maps the display strings returned by GetAuditHistory
// (which calls audit.FormatOperation server-side) to dropdown labels. Keys must
// match what the API returns, not the raw ledger values. Unknown operations fall
// through to the raw string via the `|| op` fallback in the dropdown.
const OPERATION_TYPE_LABELS: Record<string, string> = {
  "TOTP": "TOTP",
  "HOTP": "HOTP",
  "Static password": "Static password",
  "Challenge-response": "Challenge-response",
  "Vault unlock": "Vault unlock",
  "Vault lock": "Vault lock",
  "Credential addition": "Credential addition",
  "Credential removal": "Credential removal",
  "Credential update": "Credential update",
  "Credential tag update": "Credential tag update",
  "Credential import": "Credential import",
  "Credential export": "Credential export",
  "Vault passphrase change": "Vault passphrase change",
  "HOTP resync": "HOTP resync",
}

function SortIcon({ col, sortCol, sortDir }: { col: SortCol; sortCol: SortCol; sortDir: SortDir }) {
  if (sortCol !== col) return <ChevronDown className="h-3 w-3 opacity-30 inline ml-0.5" />
  return sortDir === "asc"
    ? <ChevronUp className="h-3 w-3 inline ml-0.5" />
    : <ChevronDown className="h-3 w-3 inline ml-0.5" />
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
  const [dockerPath, setDockerPath] = useState("")

  // Sorting state
  const [sortCol, setSortCol] = useState<SortCol>("timestamp")
  const [sortDir, setSortDir] = useState<SortDir>("desc")

  // Filter state
  const [opFilter, setOpFilter] = useState("")
  const [fromDate, setFromDate] = useState<Date | undefined>(undefined)
  const [toDate, setToDate] = useState<Date | undefined>(undefined)

  // Pagination state
  const [page, setPage] = useState(0)

  const [copiedHash, setCopiedHash] = useState<string | null>(null)
  const [copyMsg, setCopyMsg] = useState("")
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleFetchHistory = useCallback(async () => {
    setLoading(true)
    setError("")
    setPage(0)
    setCopiedHash(null)
    try {
      const records = await App.GetAuditHistory()
      setHistory(records || [])
    } catch (err) {
      setError(formatError(err, "Failed to fetch audit data"))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) {
      setError("")
      setVerifyResult(null)
      App.GetAuditDockerPath().then((p) => setDockerPath(p ?? "")).catch(() => setDockerPath(""))
      handleFetchHistory()
    }
  }, [open, handleFetchHistory])

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

  function toggleSort(col: SortCol) {
    if (sortCol === col) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"))
    } else {
      setSortCol(col)
      setSortDir(col === "timestamp" ? "desc" : "asc")
    }
    setPage(0)
  }

  function copyHash(hash: string) {
    navigator.clipboard.writeText(hash).then(() => {
      setCopiedHash(hash)
      setCopyMsg("Hash copied to clipboard")
      if (copyTimerRef.current !== null) clearTimeout(copyTimerRef.current)
      copyTimerRef.current = setTimeout(() => {
        setCopyMsg("")
        setCopiedHash(null)
        copyTimerRef.current = null
      }, 2000)
    }).catch(() => {
      // Keep the error visible for 10 seconds and show the hash as a fallback
      // so users can copy it manually if the clipboard is unavailable.
      setError(`Failed to copy hash to clipboard. Hash: ${hash}`)
      setTimeout(() => setError(""), 10000)
    })
  }

  // Apply filters
  const filtered = useMemo(() => history.filter((r) => {
    // When no filter selected, hide lock/unlock events by default
    if (!opFilter) {
      const op = r.operation.toLowerCase()
      if (op === "vault unlock" || op === "vault lock") return false
    }
    // Case-insensitive operation type filter
    if (opFilter && r.operation.toLowerCase() !== opFilter.toLowerCase()) return false
    if (fromDate) {
      const from = new Date(fromDate)
      from.setHours(0, 0, 0, 0)
      if (new Date(r.timestamp * 1000) < from) return false
    }
    if (toDate) {
      const to = new Date(toDate)
      to.setHours(23, 59, 59, 999)
      if (new Date(r.timestamp * 1000) > to) return false
    }
    return true
  }), [history, opFilter, fromDate, toDate])

  // Apply sort
  const sorted = useMemo(() => [...filtered].sort((a, b) => {
    let cmp = 0
    switch (sortCol) {
      case "operation": cmp = a.operation.localeCompare(b.operation); break
      case "label":     cmp = (a.label ?? "").localeCompare(b.label ?? ""); break
      case "hash":      cmp = a.hash_value.localeCompare(b.hash_value); break
      case "timestamp": cmp = a.timestamp - b.timestamp; break
    }
    return sortDir === "asc" ? cmp : -cmp
  }), [filtered, sortCol, sortDir])

  // Paginate
  const totalPages = Math.ceil(sorted.length / PAGE_SIZE)
  const pageRows = sorted.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  if (!open) return null

  return (
    <>
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
        <div className="bg-background border rounded-lg shadow-lg w-full max-w-3xl max-h-[85vh] overflow-hidden flex flex-col">
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
            {!dockerPath && history.length === 0 && !verifyResult && (
              <p className="text-sm text-muted-foreground">
                Audit logging was not enabled during vault creation.
              </p>
            )}

            {/* Action buttons */}
            <div className="flex items-center gap-2">
              <Button
                onClick={handleFetchHistory}
                disabled={loading}
                variant="outline"
                size="sm"
              >
                Refresh
              </Button>
              <Button
                onClick={handleVerify}
                disabled={loading}
                variant="outline"
                size="sm"
              >
                Verify integrity
              </Button>
              {copyMsg && (
                <span className="ml-auto text-xs" style={{ color: "var(--cinnabar)" }}>{copyMsg}</span>
              )}
            </div>

            {error && (
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
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400" />
                      <p className="text-sm font-semibold text-red-700 dark:text-red-300">
                        TAMPERING DETECTED
                      </p>
                    </div>
                    {verifyResult.faults && verifyResult.faults.length > 0 && (
                      <ul className="ml-6 space-y-0.5">
                        {verifyResult.faults.map((f, i) => (
                          <li key={i} className="text-sm text-red-700 dark:text-red-300">
                            {formatFault(f)}
                          </li>
                        ))}
                      </ul>
                    )}
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

                {/* Filters toolbar */}
                <div className="flex flex-wrap items-center gap-2">
                  <select
                    className="text-xs border rounded px-2 py-1 bg-background min-w-[11.5rem]"
                    value={opFilter}
                    onChange={(e) => { setOpFilter(e.target.value); setPage(0) }}
                    aria-label="Filter by operation type"
                  >
                    <option value="">All (lock/unlock hidden)</option>
                    {Array.from(new Set(history.map((r) => r.operation)))
                      .sort()
                      .map((op) => (
                        <option key={op} value={op}>
                          {OPERATION_TYPE_LABELS[op] || op}
                        </option>
                      ))}
                  </select>
                  <DatePicker
                    value={fromDate}
                    onChange={(d) => { setFromDate(d); setPage(0) }}
                    placeholder="From date"
                    aria-label="From date"
                  />
                  <span className="text-xs text-muted-foreground">–</span>
                  <DatePicker
                    value={toDate}
                    onChange={(d) => { setToDate(d); setPage(0) }}
                    placeholder="To date"
                    aria-label="To date"
                  />
                  <span className="text-xs text-muted-foreground ml-auto">
                    {filtered.length} of {history.length} events
                  </span>
                </div>

                {/* History table */}
                <div className="space-y-1">
                  <div className="border rounded-md overflow-x-auto">
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b bg-muted/50">
                          <th
                            className="text-left p-2 cursor-pointer select-none w-[22%]"
                            onClick={() => toggleSort("operation")}
                          >
                            Operation <SortIcon col="operation" sortCol={sortCol} sortDir={sortDir} />
                          </th>
                          <th
                            className="text-left p-2 cursor-pointer select-none w-[22%]"
                            onClick={() => toggleSort("label")}
                          >
                            Label <SortIcon col="label" sortCol={sortCol} sortDir={sortDir} />
                          </th>
                          <th
                            className="text-left p-2 cursor-pointer select-none w-[22%]"
                            onClick={() => toggleSort("timestamp")}
                          >
                            Timestamp <SortIcon col="timestamp" sortCol={sortCol} sortDir={sortDir} />
                          </th>
                          <th
                            className="text-left p-2 cursor-pointer select-none w-[34%]"
                            onClick={() => toggleSort("hash")}
                          >
                            Hash <SortIcon col="hash" sortCol={sortCol} sortDir={sortDir} />
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {pageRows.map((record, i) => {
                          const justCopied = copiedHash === record.hash_value
                          return (
                            <tr key={i} className="border-b last:border-0">
                              <td className="p-2">{record.operation}</td>
                              <td className="p-2">{record.label}</td>
                              <td className="p-2 text-muted-foreground">
                                {record.timestamp ? new Date(record.timestamp * 1000).toLocaleString() : "\u2014"}
                              </td>
                              <td className="p-2 font-mono">
                                <button
                                  type="button"
                                  className="text-left focus:outline-none cursor-pointer"
                                  style={justCopied ? { color: "var(--cinnabar)" } : undefined}
                                  title="Click to copy full hash"
                                  onClick={() => copyHash(record.hash_value)}
                                >
                                  {record.hash_value.length > 10 ? record.hash_value.slice(0, 10) + "…" : record.hash_value}
                                </button>
                              </td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  </div>

                  {/* Pagination controls */}
                  {totalPages > 1 && (
                    <div className="flex items-center justify-end gap-2 pt-1">
                      <span className="text-xs text-muted-foreground">
                        Page {page + 1} of {totalPages}
                      </span>
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-6 w-6"
                        disabled={page === 0}
                        onClick={() => setPage((p) => p - 1)}
                        aria-label="Previous page"
                      >
                        <ChevronLeft className="h-3 w-3" />
                      </Button>
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-6 w-6"
                        disabled={page >= totalPages - 1}
                        onClick={() => setPage((p) => p + 1)}
                        aria-label="Next page"
                      >
                        <ChevronRight className="h-3 w-3" />
                      </Button>
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </>
  )
}
