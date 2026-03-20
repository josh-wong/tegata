import { cn } from "@/lib/utils"

interface StrengthMeterProps {
  passphrase: string
}

type Tier = { label: string; color: string; width: string }

function getTier(length: number): Tier {
  if (length < 8)
    return { label: "Too short", color: "bg-red-500", width: "w-1/4" }
  if (length < 12)
    return { label: "Weak", color: "bg-orange-500", width: "w-1/2" }
  if (length < 16)
    return { label: "Fair", color: "bg-yellow-500", width: "w-3/4" }
  return { label: "Strong", color: "bg-green-500", width: "w-full" }
}

export function StrengthMeter({ passphrase }: StrengthMeterProps) {
  const tier = getTier(passphrase.length)

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
