import { type FormEvent, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { App } from "@/lib/wails"
import { formatError } from "@/lib/utils"
import type { Credential } from "@/lib/types"

interface EditCredentialDialogProps {
  credential: Credential | null
  open: boolean
  onClose: () => void
  onUpdated: () => void
}

export function EditCredentialDialog({ credential, open, onClose, onUpdated }: EditCredentialDialogProps) {
  const [label, setLabel] = useState("")
  const [issuer, setIssuer] = useState("")
  const [tags, setTags] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  // Pre-populate form when credential changes
  useEffect(() => {
    if (credential) {
      setLabel(credential.label)
      setIssuer(credential.issuer ?? "")
      setTags((credential.tags ?? []).join(", "))
      setError("")
    }
  }, [credential])

  if (!open || !credential) return null

  function reset() {
    setLabel("")
    setIssuer("")
    setTags("")
    setError("")
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()

    const trimmedLabel = label.trim()
    if (!trimmedLabel) {
      setError("Label is required")
      return
    }

    const tagList = tags
      .split(",")
      .map((t) => t.trim())
      .filter((t) => t !== "")

    // Check for duplicate tags
    const tagSet = new Set(tagList)
    if (tagSet.size !== tagList.length) {
      setError("Duplicate tags found")
      return
    }

    setLoading(true)
    setError("")
    try {
      await App.EditCredential(credential!.id, trimmedLabel, issuer, tagList)
      reset()
      onUpdated()
      onClose()
    } catch (err) {
      setError(formatError(err, "Failed to update credential"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Credential</DialogTitle>
          <DialogDescription>
            Update the label, issuer, and tags for this credential.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="block text-sm font-medium">Label</label>
            <Input
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="e.g. GitHub"
              disabled={loading}
              autoFocus
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium">Issuer (optional)</label>
            <Input
              type="text"
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
              placeholder="e.g. GitHub Inc"
              disabled={loading}
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium">Tags (comma-separated, optional)</label>
            <Input
              type="text"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="e.g. work, personal, 2fa"
              disabled={loading}
            />
          </div>

          {error && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="flex gap-2 justify-end">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={loading}
            >
              {loading ? "Updating..." : "Update"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
