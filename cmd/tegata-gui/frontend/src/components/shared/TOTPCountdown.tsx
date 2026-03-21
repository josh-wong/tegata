import { useEffect, useState } from "react"
import { cn } from "@/lib/utils"

interface TOTPCountdownProps {
  code: string
  remaining: number
  period: number
  onExpired: () => void
}

function ringColor(seconds: number): string {
  if (seconds > 10) return "text-green-500"
  if (seconds > 5) return "text-yellow-500"
  return "text-red-500"
}

export function TOTPCountdown({
  code,
  remaining,
  period,
  onExpired,
}: TOTPCountdownProps) {
  const [secondsLeft, setSecondsLeft] = useState(remaining)

  useEffect(() => {
    setSecondsLeft(remaining)
  }, [remaining])

  useEffect(() => {
    const interval = setInterval(() => {
      setSecondsLeft((prev) => {
        if (prev <= 1) {
          onExpired()
          return 0
        }
        return prev - 1
      })
    }, 1000)
    return () => clearInterval(interval)
  }, [onExpired])

  const radius = 20
  const circumference = 2 * Math.PI * radius
  const safeSeconds = Math.max(0, secondsLeft ?? 0)
  const progress = period > 0 ? safeSeconds / period : 0
  const dashOffset = circumference * (1 - progress)

  const formatted =
    code.length === 6
      ? `${code.slice(0, 3)} ${code.slice(3)}`
      : code.length === 8
        ? `${code.slice(0, 4)} ${code.slice(4)}`
        : code

  return (
    <div className="flex items-center gap-4">
      <div className="relative h-12 w-12 shrink-0">
        <svg className="h-12 w-12 -rotate-90" viewBox="0 0 48 48">
          <circle
            cx="24"
            cy="24"
            r={radius}
            fill="none"
            stroke="currentColor"
            strokeWidth="3"
            className="text-muted/30"
          />
          <circle
            cx="24"
            cy="24"
            r={radius}
            fill="none"
            stroke="currentColor"
            strokeWidth="3"
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={dashOffset}
            className={cn("transition-all duration-1000 ease-linear", ringColor(safeSeconds))}
          />
        </svg>
        <span className="absolute inset-0 flex items-center justify-center text-xs font-medium">
          {safeSeconds}
        </span>
      </div>
      <span className="font-mono text-3xl font-bold tracking-wider">
        {formatted}
      </span>
    </div>
  )
}
