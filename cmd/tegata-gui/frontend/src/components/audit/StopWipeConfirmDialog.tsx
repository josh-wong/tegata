import { useState } from "react"
import { Loader2, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { App } from "@/lib/wails"
import { formatError } from "@/lib/utils"

interface StopWipeConfirmDialogProps {
  open: boolean
  onClose: () => void
  onWipeComplete: () => void
}

export function StopWipeConfirmDialog({
  open,
  onClose,
  onWipeComplete,
}: StopWipeConfirmDialogProps) {
  const [confirmText, setConfirmText] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  function handleClose() {
    setConfirmText("")
    setError("")
    setLoading(false)
    onClose()
  }

  async function handleConfirm() {
    if (confirmText !== "DELETE") return
    setLoading(true)
    setError("")
    try {
      await App.StopAuditServer(true)
      setConfirmText("")
      setLoading(false)
      onWipeComplete()
      onClose()
    } catch (err) {
      setError(formatError(err, "Failed to delete audit history"))
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-destructive">
            <Trash2 className="h-5 w-5" />
            Permanently delete audit history?
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <p className="text-sm text-muted-foreground">
            This permanently deletes all audit history and cannot be undone.
            Type DELETE to confirm.
          </p>
          {loading && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Deleting history and restarting the audit server. This may take up to 30 seconds...
            </div>
          )}
          <div className="space-y-1">
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder="DELETE"
              autoComplete="off"
              data-testid="wipe-confirm-input"
            />
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={loading}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={confirmText !== "DELETE" || loading}
          >
            {loading ? (
              <>
                <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                Deleting...
              </>
            ) : "Delete permanently"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
