import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import type { Credential } from "@/lib/types"

interface DetailPanelProps {
  credential: Credential | null
}

export function DetailPanel({ credential }: DetailPanelProps) {
  if (!credential) {
    return (
      <main className="flex flex-1 items-center justify-center bg-background">
        <p className="text-muted-foreground">Select a credential</p>
      </main>
    )
  }

  return (
    <main className="flex flex-1 flex-col bg-background p-6">
      <div className="mb-4">
        <h2 className="text-xl font-semibold">{credential.label}</h2>
        {credential.issuer && (
          <p className="text-sm text-muted-foreground">{credential.issuer}</p>
        )}
      </div>

      <Separator />

      <dl className="mt-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
        <dt className="font-medium text-muted-foreground">Type</dt>
        <dd className="uppercase">{credential.type}</dd>

        <dt className="font-medium text-muted-foreground">Algorithm</dt>
        <dd>{credential.algorithm}</dd>

        <dt className="font-medium text-muted-foreground">Digits</dt>
        <dd>{credential.digits}</dd>

        {credential.type === "totp" && (
          <>
            <dt className="font-medium text-muted-foreground">Period</dt>
            <dd>{credential.period}s</dd>
          </>
        )}

        {credential.type === "hotp" && (
          <>
            <dt className="font-medium text-muted-foreground">Counter</dt>
            <dd>{credential.counter}</dd>
          </>
        )}
      </dl>

      {credential.tags.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-1">
          {credential.tags.map((tag) => (
            <Badge key={tag} variant="secondary">
              {tag}
            </Badge>
          ))}
        </div>
      )}

      {credential.notes && (
        <p className="mt-4 text-sm text-muted-foreground">
          {credential.notes}
        </p>
      )}
    </main>
  )
}
