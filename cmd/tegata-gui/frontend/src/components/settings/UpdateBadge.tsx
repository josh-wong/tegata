import { useEffect, useRef } from "react"
import { App } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"

interface UpdateBadgeProps {
  updateInfo: UpdateInfo | null
  onUpdateFound: (info: UpdateInfo) => void
}

export function UpdateBadge({ updateInfo, onUpdateFound }: UpdateBadgeProps) {
  const checkedRef = useRef(false)

  useEffect(() => {
    if (checkedRef.current) return
    checkedRef.current = true
    App.CheckForUpdate()
      .then((info) => {
        if (info) {
          onUpdateFound(info)
        }
      })
      .catch(() => {})
  }, [onUpdateFound])

  if (!updateInfo) return null

  return (
    <span className="absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-primary" />
  )
}
