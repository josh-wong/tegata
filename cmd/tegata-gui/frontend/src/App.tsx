import { useState } from "react"
import { Header } from "@/components/layout/Header"
import { Sidebar } from "@/components/layout/Sidebar"
import { DetailPanel } from "@/components/layout/DetailPanel"
import type { Credential } from "@/lib/types"

function App() {
  const [credentials] = useState<Credential[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState("")

  const selected = credentials.find((c) => c.id === selectedId) ?? null

  return (
    <div className="flex h-screen flex-col bg-background text-foreground">
      <Header onSettingsClick={() => {}} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          credentials={credentials}
          selectedId={selectedId}
          onSelect={setSelectedId}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
        />
        <DetailPanel credential={selected} />
      </div>
    </div>
  )
}

export default App
