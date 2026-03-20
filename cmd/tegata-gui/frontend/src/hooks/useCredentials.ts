import { useCallback, useMemo, useState } from "react"
import { App } from "@/lib/wails"
import type { Credential } from "@/lib/types"

// Go serializes nil slices/empty strings as null. Normalize at the boundary.
function normalizeCred(c: Credential): Credential {
  return {
    ...c,
    tags: c.tags ?? [],
    notes: c.notes ?? "",
    issuer: c.issuer ?? "",
    algorithm: c.algorithm ?? "SHA1",
    digits: c.digits || 6,
    period: c.period || 30,
    counter: c.counter ?? 0,
  }
}

export function useCredentials() {
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState("")

  const refresh = useCallback(async () => {
    try {
      const list = await App.ListCredentials()
      setCredentials((list ?? []).map(normalizeCred))
    } catch {
      setCredentials([])
    }
  }, [])

  const filteredCredentials = useMemo(() => {
    if (!searchQuery) return credentials
    const q = searchQuery.toLowerCase()
    return credentials.filter(
      (c) =>
        c.label.toLowerCase().includes(q) ||
        (c.issuer ?? "").toLowerCase().includes(q),
    )
  }, [credentials, searchQuery])

  const selectedCredential = useMemo(
    () => credentials.find((c) => c.id === selectedId) ?? null,
    [credentials, selectedId],
  )

  return {
    credentials,
    filteredCredentials,
    selectedId,
    selectedCredential,
    searchQuery,
    refresh,
    select: setSelectedId,
    search: setSearchQuery,
  } as const
}
