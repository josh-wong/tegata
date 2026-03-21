import { useEffect, useRef, useState } from "react"
import { ChevronRight, Copy, Key, Plus, Search, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { Credential } from "@/lib/types"

interface SidebarProps {
  credentials: Credential[]
  selectedId: string | null
  onSelect: (id: string) => void
  searchQuery: string
  onSearchChange: (q: string) => void
  onAddClick: () => void
  onCopyCode: (label: string) => void
  onCopyPassword: (label: string) => void
  onRemove: (id: string) => void
}

interface ContextMenuState {
  x: number
  y: number
  credential: Credential
}

function groupByTag(credentials: Credential[]) {
  const groups = new Map<string, Credential[]>()
  for (const cred of credentials) {
    const t = cred.tags ?? []
    const tags = t.length > 0 ? t : ["[Untagged]"]
    for (const tag of tags) {
      const list = groups.get(tag) ?? []
      list.push(cred)
      groups.set(tag, list)
    }
  }
  return groups
}

const typeBadgeColor: Record<string, string> = {
  totp: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  hotp: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  static: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  "challenge-response": "bg-purple-500/15 text-purple-700 dark:text-purple-400",
}

const typeBadgeLabel: Record<string, string> = {
  "challenge-response": "CR",
}

export function Sidebar({
  credentials,
  selectedId,
  onSelect,
  searchQuery,
  onSearchChange,
  onAddClick,
  onCopyCode,
  onCopyPassword,
  onRemove,
}: SidebarProps) {
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [ctxMenu, setCtxMenu] = useState<ContextMenuState | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  // Close context menu on outside click or Escape.
  useEffect(() => {
    if (!ctxMenu) return
    function handleClose(e: MouseEvent | KeyboardEvent) {
      if (e instanceof KeyboardEvent && e.key !== "Escape") return
      setCtxMenu(null)
    }
    document.addEventListener("click", handleClose)
    document.addEventListener("keydown", handleClose)
    return () => {
      document.removeEventListener("click", handleClose)
      document.removeEventListener("keydown", handleClose)
    }
  }, [ctxMenu])

  const filtered = credentials.filter(
    (c) =>
      c.label.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (c.issuer ?? "").toLowerCase().includes(searchQuery.toLowerCase()),
  )

  const groups = groupByTag(filtered)

  function toggleGroup(tag: string) {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(tag)) {
        next.delete(tag)
      } else {
        next.add(tag)
      }
      return next
    })
  }

  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-border bg-sidebar">
      <div className="p-3">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Search credentials..."
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              className="h-8 pl-8 text-sm"
            />
          </div>
          <Button variant="outline" size="icon" className="h-8 w-8 shrink-0" onClick={onAddClick}>
            <Plus className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <ScrollArea className="flex-1">
        <div className="px-2 pb-2">
          {Array.from(groups.entries()).map(([tag, creds]) => (
            <div key={tag} className="mb-1">
              <button
                onClick={() => toggleGroup(tag)}
                className="flex w-full items-center gap-1 rounded px-2 py-1 text-xs font-medium uppercase tracking-wider text-muted-foreground hover:bg-accent"
              >
                <ChevronRight
                  className={cn(
                    "h-3 w-3 transition-transform",
                    !collapsed.has(tag) && "rotate-90",
                  )}
                />
                {tag}
              </button>

              {!collapsed.has(tag) &&
                creds.map((cred) => (
                  <button
                    key={cred.id}
                    onClick={() => onSelect(cred.id)}
                    onContextMenu={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      setCtxMenu({ x: e.clientX, y: e.clientY, credential: cred })
                    }}
                    className={cn(
                      "flex w-full items-center justify-between rounded px-3 py-1.5 text-left text-sm hover:bg-accent",
                      selectedId === cred.id && "bg-accent",
                    )}
                  >
                    <div className="min-w-0">
                      <div className="truncate font-medium">{cred.label}</div>
                      {cred.issuer && (
                        <div className="truncate text-xs text-muted-foreground">
                          {cred.issuer}
                        </div>
                      )}
                    </div>
                    <Badge
                      variant="secondary"
                      className={cn(
                        "ml-2 shrink-0 text-[10px] uppercase",
                        typeBadgeColor[cred.type],
                      )}
                    >
                      {typeBadgeLabel[cred.type] ?? cred.type}
                    </Badge>
                  </button>
                ))}
            </div>
          ))}

          {filtered.length === 0 && (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">
              {credentials.length === 0
                ? "No credentials yet"
                : "No matches found"}
            </p>
          )}
        </div>
      </ScrollArea>

      {ctxMenu && (
        <div
          ref={menuRef}
          className="fixed z-50 min-w-[160px] rounded-md border border-border bg-popover p-1 shadow-md"
          style={{ left: ctxMenu.x, top: ctxMenu.y }}
        >
          {ctxMenu.credential.type === "totp" && (
            <button
              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
              onClick={() => { onCopyCode(ctxMenu.credential.label); setCtxMenu(null) }}
            >
              <Copy className="h-3.5 w-3.5" /> Copy code
            </button>
          )}
          {ctxMenu.credential.type === "static" && (
            <button
              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
              onClick={() => { onCopyPassword(ctxMenu.credential.label); setCtxMenu(null) }}
            >
              <Key className="h-3.5 w-3.5" /> Copy password
            </button>
          )}
          <button
            className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive hover:bg-accent"
            onClick={() => { onRemove(ctxMenu.credential.id); setCtxMenu(null) }}
          >
            <Trash2 className="h-3.5 w-3.5" /> Remove
          </button>
        </div>
      )}
    </aside>
  )
}
