import { useEffect, useState } from "react"
import { App } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"

interface UpdateBadgeProps {
  updateInfo: UpdateInfo | null
  onUpdateFound: (info: UpdateInfo) => void
}

export function UpdateBadge({ updateInfo, onUpdateFound }: UpdateBadgeProps) {
  const [checked, setChecked] = useState(false)

  useEffect(() => {
    if (checked) return
    setChecked(true)
    App.CheckForUpdate()
      .then((info) => {
        if (info) {
          onUpdateFound(info)
        }
      })
      .catch(() => {})
  }, [checked, onUpdateFound])

  if (!updateInfo) return null

  return (
    <span className="absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-primary" />
  )
}
