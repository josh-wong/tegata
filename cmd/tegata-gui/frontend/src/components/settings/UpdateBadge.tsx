import { useEffect, useState } from "react"
import { App } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"

interface UpdateBadgeProps {
  onUpdateFound: (info: UpdateInfo) => void
}

export function UpdateBadge({ onUpdateFound }: UpdateBadgeProps) {
  const [hasUpdate, setHasUpdate] = useState(false)

  useEffect(() => {
    App.CheckForUpdate()
      .then((info) => {
        if (info) {
          setHasUpdate(true)
          onUpdateFound(info)
        }
      })
      .catch(() => {})
  }, [onUpdateFound])

  if (!hasUpdate) return null

  return (
    <span className="absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-primary" />
  )
}
