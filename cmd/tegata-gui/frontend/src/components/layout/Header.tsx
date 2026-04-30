import { FolderSync, Settings, Shield } from "lucide-react"
import { Button } from "@/components/ui/button"
import { UpdateBadge } from "@/components/settings/UpdateBadge"
import type { UpdateInfo } from "@/lib/types"

interface HeaderProps {
  onSettingsClick: () => void
  onAuditClick?: () => void
  onSwitchVault: () => void
  onUpdateFound: (info: UpdateInfo) => void
  vaultPath?: string
}

function truncateVaultPath(path: string, maxWidth: number): string {
  if (path.length <= maxWidth) {
    return path
  }
  if (maxWidth < 10) {
    return "vault"
  }
  const ellipsis = "..."
  const usableWidth = maxWidth - ellipsis.length
  const startWidth = Math.floor(usableWidth / 2)
  const endWidth = usableWidth - startWidth
  return path.slice(0, startWidth) + ellipsis + path.slice(-endWidth)
}

export function Header({ onSettingsClick, onAuditClick, onSwitchVault, onUpdateFound, vaultPath }: HeaderProps) {
  const truncatedPath = vaultPath ? truncateVaultPath(vaultPath, 100) : ""

  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-card px-4">
      <div className="flex flex-col">
        <h1 className="text-lg font-semibold tracking-tight text-primary">
          Tegata
        </h1>
        {truncatedPath && (
          <p className="text-xs text-muted-foreground" title={vaultPath}>
            {truncatedPath}
          </p>
        )}
      </div>

      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={onSwitchVault}
        >
          <FolderSync className="h-4 w-4" />
        </Button>

        {onAuditClick && (
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={onAuditClick}
          >
            <Shield className="h-4 w-4" />
          </Button>
        )}

        <div className="relative">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={onSettingsClick}
          >
            <Settings className="h-4 w-4" />
          </Button>
          <UpdateBadge onUpdateFound={onUpdateFound} />
        </div>
      </div>
    </header>
  )
}
