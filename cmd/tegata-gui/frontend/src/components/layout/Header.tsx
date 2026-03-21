import { useCallback } from "react"
import { FolderSync, Settings } from "lucide-react"
import { Button } from "@/components/ui/button"
import { UpdateBadge } from "@/components/settings/UpdateBadge"
import type { UpdateInfo } from "@/lib/types"

interface HeaderProps {
  onSettingsClick: () => void
  onSwitchVault: () => void
  onUpdateFound: (info: UpdateInfo) => void
}

export function Header({ onSettingsClick, onSwitchVault, onUpdateFound }: HeaderProps) {
  const stableOnUpdateFound = useCallback(onUpdateFound, [onUpdateFound])

  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-card px-4">
      <h1 className="text-lg font-semibold tracking-tight text-primary">
        Tegata
      </h1>

      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={onSwitchVault}
        >
          <FolderSync className="h-4 w-4" />
        </Button>

        <div className="relative">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={onSettingsClick}
          >
            <Settings className="h-4 w-4" />
          </Button>
          <UpdateBadge onUpdateFound={stableOnUpdateFound} />
        </div>
      </div>
    </header>
  )
}
