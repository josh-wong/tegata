import { useRef, useState, useEffect } from "react"
import { ChevronDown } from "lucide-react"
import type { VaultLocation } from "@/lib/types"

interface VaultSelectorProps {
  vaultPath: string | null
  vaultLocations: VaultLocation[]
  onSelectVault: (path: string) => void
  disabled?: boolean
}

export function VaultSelector({
  vaultPath,
  vaultLocations,
  onSelectVault,
  disabled,
}: VaultSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  const currentVault = vaultLocations.find((v) => v.path === vaultPath)
  const displayText = currentVault
    ? `${currentVault.driveName} — ${currentVault.path.split("/").pop() || "vault"}`
    : "Select a vault"

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside)
      return () => document.removeEventListener("mousedown", handleClickOutside)
    }
  }, [isOpen])

  // Handle keyboard navigation
  function handleKeyDown(e: React.KeyboardEvent) {
    if (!isOpen) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault()
        setIsOpen(true)
      }
      return
    }

    switch (e.key) {
      case "Escape":
        setIsOpen(false)
        buttonRef.current?.focus()
        break
      case "ArrowDown":
        e.preventDefault()
        setHighlightedIndex((prev) =>
          prev < vaultLocations.length - 1 ? prev + 1 : prev
        )
        break
      case "ArrowUp":
        e.preventDefault()
        setHighlightedIndex((prev) => (prev > 0 ? prev - 1 : prev))
        break
      case "Enter":
        e.preventDefault()
        onSelectVault(vaultLocations[highlightedIndex].path)
        setIsOpen(false)
        break
    }
  }

  function handleSelect(path: string) {
    onSelectVault(path)
    setIsOpen(false)
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={buttonRef}
        onClick={() => setIsOpen(!isOpen)}
        onKeyDown={handleKeyDown}
        disabled={disabled}
        className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-left flex items-center justify-between hover:bg-accent hover:text-accent-foreground disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        <span className="truncate">{displayText}</span>
        <ChevronDown
          className="h-4 w-4 shrink-0 opacity-50 transition-transform"
          style={{
            transform: isOpen ? "rotate(180deg)" : "rotate(0deg)",
          }}
        />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 right-0 mt-1 z-50 rounded-md border border-input bg-popover shadow-lg">
          <div className="max-h-60 overflow-y-auto">
            {vaultLocations.map((loc, index) => {
              const filename = loc.path.split("/").pop() || "vault"
              const isHighlighted = index === highlightedIndex
              const isSelected = loc.path === vaultPath

              return (
                <button
                  key={loc.path}
                  onClick={() => handleSelect(loc.path)}
                  onMouseEnter={() => setHighlightedIndex(index)}
                  onKeyDown={handleKeyDown}
                  className={`w-full text-left px-3 py-2 flex items-start justify-between transition-colors ${
                    isHighlighted
                      ? "bg-accent text-accent-foreground"
                      : "hover:bg-accent/50"
                  }`}
                >
                  <div className="flex flex-col flex-1 min-w-0">
                    <span className="font-medium">{loc.driveName}</span>
                    <span className="text-xs text-muted-foreground truncate">
                      {filename}
                    </span>
                  </div>
                  {isSelected && (
                    <span className="ml-2 text-accent-foreground">✓</span>
                  )}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
