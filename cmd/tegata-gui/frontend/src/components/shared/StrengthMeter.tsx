import { cn } from "@/lib/utils"

interface StrengthMeterProps {
  passphrase: string
}

type Tier = { label: string; color: string; width: string }

function charClasses(s: string): number {
  let classes = 0
  if (/[a-z]/.test(s)) classes++
  if (/[A-Z]/.test(s)) classes++
  if (/[0-9]/.test(s)) classes++
  if (/[^a-zA-Z0-9]/.test(s)) classes++
  return classes
}

function getTier(passphrase: string): Tier {
  const len = passphrase.length
  if (len < 8)
    return { label: "Too short", color: "bg-red-500", width: "w-1/4" }
  const classes = charClasses(passphrase)
  const score = len + classes * 3
  if (score < 15)
    return { label: "Weak", color: "bg-orange-500", width: "w-1/2" }
  if (score < 22)
    return { label: "Fair", color: "bg-yellow-500", width: "w-3/4" }
  return { label: "Strong", color: "bg-green-500", width: "w-full" }
}

export function StrengthMeter({ passphrase }: StrengthMeterProps) {
  const tier = getTier(passphrase)

  return (
    <div className="space-y-1">
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full transition-all duration-300",
            tier.color,
            tier.width,
          )}
        />
      </div>
      <p className="text-xs text-muted-foreground">{tier.label}</p>
    </div>
  )
}
