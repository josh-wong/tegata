import { useEffect, useRef, useState } from "react"
import { createPortal } from "react-dom"
import { format } from "date-fns"
import { CalendarIcon } from "lucide-react"
import { Calendar } from "@/components/ui/calendar"
import { cn } from "@/lib/utils"

interface DatePickerProps {
  value: Date | undefined
  onChange: (date: Date | undefined) => void
  placeholder?: string
  className?: string
  "aria-label"?: string
}

export function DatePicker({
  value,
  onChange,
  placeholder = "Pick a date",
  className,
  "aria-label": ariaLabel,
}: DatePickerProps) {
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState({ top: 0, left: 0 })
  const [month, setMonth] = useState<Date>(value ?? new Date())
  const triggerRef = useRef<HTMLButtonElement>(null)

  function handleTriggerClick() {
    if (!triggerRef.current) return
    const rect = triggerRef.current.getBoundingClientRect()
    setPosition({
      top: rect.bottom + window.scrollY + 4,
      left: rect.left + window.scrollX,
    })
    setMonth(value ?? new Date())
    setOpen((prev) => !prev)
  }

  function handleSelect(date: Date | undefined) {
    onChange(date)
    setOpen(false)
  }

  // Close on Escape
  useEffect(() => {
    if (!open) return
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false)
    }
    document.addEventListener("keydown", onKeyDown)
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [open])

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        onClick={handleTriggerClick}
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="dialog"
        className={cn(
          "inline-flex items-center gap-1.5 text-xs border rounded px-2 py-1 bg-background",
          "hover:bg-accent hover:text-accent-foreground",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          !value && "text-muted-foreground",
          className,
        )}
      >
        <CalendarIcon className="h-3.5 w-3.5 shrink-0" />
        {value ? format(value, "MMM d, yyyy") : placeholder}
      </button>

      {open &&
        createPortal(
          <>
            {/* Transparent backdrop — clicking outside the popup closes it */}
            <div
              style={{ position: "fixed", inset: 0, zIndex: 9998 }}
              onClick={() => setOpen(false)}
            />
            {/* Calendar popup — sits above the backdrop */}
            <div
              role="dialog"
              style={{ position: "absolute", top: position.top, left: position.left, zIndex: 9999 }}
              className="rounded-md border bg-background shadow-md"
            >
              <Calendar
                mode="single"
                selected={value}
                month={month}
                onMonthChange={setMonth}
                onSelect={handleSelect}
                autoFocus
              />
            </div>
          </>,
          document.body,
        )}
    </>
  )
}
