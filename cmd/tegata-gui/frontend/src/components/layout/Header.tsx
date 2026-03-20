import { useCallback } from "react"
import { Moon, Settings, Sun, Monitor } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useTheme } from "@/hooks/useTheme"
import { UpdateBadge } from "@/components/settings/UpdateBadge"
import type { UpdateInfo } from "@/lib/types"

interface HeaderProps {
  onSettingsClick: () => void
  onUpdateFound: (info: UpdateInfo) => void
}

export function Header({ onSettingsClick, onUpdateFound }: HeaderProps) {
  const stableOnUpdateFound = useCallback(onUpdateFound, [onUpdateFound])
  const { theme, setTheme } = useTheme()

  const themeIcon =
    theme === "dark" ? (
      <Moon className="h-4 w-4" />
    ) : theme === "light" ? (
      <Sun className="h-4 w-4" />
    ) : (
      <Monitor className="h-4 w-4" />
    )

  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-card px-4">
      <h1 className="text-lg font-semibold tracking-tight text-primary">
        Tegata
      </h1>

      <div className="flex items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" className="h-8 w-8">
                {themeIcon}
              </Button>
            }
          />
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => setTheme("light")}>
              <Sun className="mr-2 h-4 w-4" /> Light
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setTheme("dark")}>
              <Moon className="mr-2 h-4 w-4" /> Dark
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setTheme("system")}>
              <Monitor className="mr-2 h-4 w-4" /> System
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

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
