import { useState } from "react"
import { Trash2 } from "lucide-react"
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
    onClose()
  }

  async function handleConfirm() {
    if (confirmText !== "yes") return
    setLoading(true)
    setError("")
    try {
      await App.StopAuditServer(true)
      setConfirmText("")
      onWipeComplete()
      onClose()
    } catch (err) {
      setError(formatError(err, "Failed to delete audit history"))
    } finally {
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
            Type &lsquo;yes&rsquo; to confirm.
          </p>
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">
              Type &lsquo;yes&rsquo; to confirm
            </label>
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder="yes"
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
            disabled={confirmText !== "yes" || loading}
          >
            Delete permanently
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
