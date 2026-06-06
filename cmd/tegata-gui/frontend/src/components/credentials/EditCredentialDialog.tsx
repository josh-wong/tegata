import { type FormEvent, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
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
  const { t } = useTranslation()
  const [label, setLabel] = useState("")
  const [issuer, setIssuer] = useState("")
  const [tags, setTags] = useState("")
  const [category, setCategory] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  // Pre-populate form when credential changes
  useEffect(() => {
    if (credential) {
      setLabel(credential.label)
      setIssuer(credential.issuer ?? "")
      setTags((credential.tags ?? []).join(", "))
      setCategory(credential.category ?? "")
      setError("")
    }
  }, [credential])

  if (!open || !credential) return null

  function reset() {
    setLabel("")
    setIssuer("")
    setTags("")
    setCategory("")
    setError("")
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()

    const trimmedLabel = label.trim()
    if (!trimmedLabel) {
      setError(t("gui.edit.errorRequired"))
      return
    }

    const tagList = tags
      .split(",")
      .map((t) => t.trim().toLowerCase())
      .filter((t) => t !== "")

    // Check for duplicate tags
    const tagSet = new Set(tagList)
    if (tagSet.size !== tagList.length) {
      setError(t("gui.edit.errorDuplicateTags"))
      return
    }

    const normalizedCategory = category.trim().toLowerCase()

    setLoading(true)
    setError("")
    try {
      await App.EditCredential(credential!.id, trimmedLabel, issuer, tagList, normalizedCategory)
      reset()
      onUpdated()
      onClose()
    } catch (err) {
      setError(formatError(err, t("gui.edit.errorUpdate")))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("gui.edit.title")}</DialogTitle>
          <DialogDescription>
            {t("gui.edit.description")}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="block text-sm font-medium">{t("gui.edit.labelField")}</label>
            <Input
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={t("gui.edit.labelPlaceholder")}
              disabled={loading}
              autoFocus
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium">{t("gui.edit.issuerField")}</label>
            <Input
              type="text"
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
              placeholder={t("gui.edit.issuerPlaceholder")}
              disabled={loading}
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium">{t("gui.edit.categoryField")}</label>
            <Input
              type="text"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              placeholder={t("gui.edit.categoryPlaceholder")}
              disabled={loading}
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium">{t("gui.edit.tagsField")}</label>
            <Input
              type="text"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder={t("gui.edit.tagsPlaceholder")}
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
              {t("gui.common.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={loading}
            >
              {loading ? t("gui.edit.updating") : t("gui.common.update")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
