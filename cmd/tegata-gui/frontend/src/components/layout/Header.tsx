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
  const chars = [...path]
  if (chars.length <= maxWidth) {
    return path
  }
  if (maxWidth < 10) {
    return "vault"
  }
  const ellipsis = "..."
  const usableWidth = maxWidth - ellipsis.length
  const startWidth = Math.floor(usableWidth / 2)
  const endWidth = usableWidth - startWidth
  return chars.slice(0, startWidth).join("") + ellipsis + chars.slice(-endWidth).join("")
}

const MAX_VAULT_PATH_DISPLAY = 100

function getVaultPathParts(path: string): { dir: string; filename: string } {
  const lastSep = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"))
  if (lastSep === -1) {
    return { dir: "", filename: path }
  }
  return {
    dir: path.slice(0, lastSep + 1),
    filename: path.slice(lastSep + 1),
  }
}

export function Header({ onSettingsClick, onAuditClick, onSwitchVault, onUpdateFound, vaultPath }: HeaderProps) {
  let displayDir = ""
  let displayFilename = ""

  if (vaultPath) {
    const { dir, filename } = getVaultPathParts(vaultPath)
    displayFilename = filename
    if (dir) {
      const maxDirWidth = MAX_VAULT_PATH_DISPLAY - [...filename].length
      displayDir = maxDirWidth >= 10 ? truncateVaultPath(dir, maxDirWidth) : ""
    }
  }

  return (
    <header className={`flex shrink-0 items-center justify-between border-b border-border bg-card px-4 ${vaultPath ? "h-16" : "h-12"}`}>
      <div className="flex flex-col">
        <h1 className="text-lg font-semibold tracking-tight text-primary">
          Tegata
        </h1>
        {vaultPath && (
          <p className="text-xs text-muted-foreground" title={vaultPath}>
            {displayDir && <span>{displayDir}</span>}
            <span className="font-semibold">{displayFilename}</span>
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
