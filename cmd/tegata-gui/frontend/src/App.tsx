import { useCallback, useEffect, useState } from "react"
import { Header } from "@/components/layout/Header"
import { Sidebar } from "@/components/layout/Sidebar"
import { CredentialDetail } from "@/components/credentials/CredentialDetail"
import { AddCredentialDialog } from "@/components/credentials/AddCredentialDialog"
import { SettingsPanel } from "@/components/settings/SettingsPanel"
import { UnlockView } from "@/components/vault/UnlockView"
import { SetupWizard } from "@/components/vault/SetupWizard"
import { LoadingSpinner } from "@/components/shared/LoadingSpinner"
import { useVault } from "@/hooks/useVault"
import { useCredentials } from "@/hooks/useCredentials"
import { useIdleTimer } from "@/hooks/useIdleTimer"
import { App as WailsApp } from "@/lib/wails"
import type { UpdateInfo } from "@/lib/types"

function App() {
  const vault = useVault()
  const creds = useCredentials()

  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)

  const handleLock = useCallback(() => {
    vault.lock()
  }, [vault])

  useIdleTimer(5 * 60 * 1000, handleLock)

  useEffect(() => {
    if (vault.view === "main") {
      creds.refresh()
    }
  }, [vault.view]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleRemove = useCallback(
    async (id: string) => {
      try {
        await WailsApp.RemoveCredential(id)
        creds.refresh()
        creds.select(null)
      } catch {
        // Error handling via toast in future
      }
    },
    [creds],
  )

  if (vault.view === "loading") {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <LoadingSpinner size="lg" message="Loading..." />
      </div>
    )
  }

  if (vault.view === "setup") {
    return (
      <SetupWizard
        vaultLocations={vault.vaultLocations}
        loading={vault.loading}
        error={vault.error}
        onCreateVault={vault.createVault}
        onOpenExisting={(path) => {
          vault.setVaultPath(path)
          vault.setView("unlock")
        }}
        onComplete={() => vault.setView("main")}
      />
    )
  }

  if (vault.view === "unlock") {
    return (
      <UnlockView
        vaultPath={vault.vaultPath}
        vaultLocations={vault.vaultLocations}
        error={vault.error}
        loading={vault.loading}
        onUnlock={vault.unlock}
        onSelectVault={vault.setVaultPath}
        onBack={() => vault.setView("setup")}
      />
    )
  }

  return (
    <div className="flex h-screen flex-col bg-background text-foreground">
      <Header
        onSettingsClick={() => setSettingsOpen(true)}
        onUpdateFound={setUpdateInfo}
      />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          credentials={creds.filteredCredentials}
          selectedId={creds.selectedId}
          onSelect={creds.select}
          searchQuery={creds.searchQuery}
          onSearchChange={creds.search}
          onAddClick={() => setAddDialogOpen(true)}
        />
        <CredentialDetail
          credential={creds.selectedCredential}
          onRemove={handleRemove}
        />
      </div>

      <AddCredentialDialog
        open={addDialogOpen}
        onClose={() => setAddDialogOpen(false)}
        onAdded={creds.refresh}
      />

      <SettingsPanel
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        updateInfo={updateInfo}
      />
    </div>
  )
}

export default App
