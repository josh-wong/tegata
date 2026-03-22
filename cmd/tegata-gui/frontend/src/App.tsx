import { Component, useCallback, useEffect, useState } from "react"
import type { ErrorInfo, ReactNode } from "react"
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
  const [setupStep, setSetupStep] = useState<1 | 2 | 3 | 4 | 5 | 6>(1)
  const [isSwitching, setIsSwitching] = useState(false)
  const [idleTimeoutMs, setIdleTimeoutMs] = useState(5 * 60 * 1000)

  useEffect(() => {
    WailsApp.GetIdleTimeoutSeconds()
      .then((s) => setIdleTimeoutMs(s * 1000))
      .catch(() => {})
  }, [settingsOpen])

  const handleLock = useCallback(() => {
    vault.lock()
  }, [vault])

  useIdleTimer(idleTimeoutMs, handleLock)

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
        if (creds.selectedId === id) creds.select(null)
      } catch (err) {
        console.error("Failed to remove credential:", err)
      }
    },
    [creds],
  )

  const handleCopyCode = useCallback(async (label: string) => {
    try {
      const result = await WailsApp.GenerateTOTP(label)
      if (result?.code) {
        await navigator.clipboard.writeText(result.code)
      }
    } catch (err) {
      console.error("Failed to copy code:", err)
    }
  }, [])

  const handleCopyPassword = useCallback(async (label: string) => {
    try {
      await WailsApp.GetStaticPassword(label)
    } catch (err) {
      console.error("Failed to copy password:", err)
    }
  }, [])

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
        initialStep={setupStep}
        onCancel={isSwitching ? () => {
          setIsSwitching(false)
          vault.setView("main")
        } : undefined}
        onCreateVault={vault.createVault}
        onOpenExisting={async (path) => {
          if (isSwitching) {
            try { await WailsApp.LockVault() } catch { /* non-critical */ }
          }
          setSetupStep(6)
          setIsSwitching(false)
          vault.setVaultPath(path)
          vault.setView("unlock")
        }}
        onComplete={() => {
          setIsSwitching(false)
          vault.setView("main")
        }}
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
        onBack={() => {
          setSetupStep(1)
          vault.setView("setup")
        }}
      />
    )
  }

  return (
    <div className="flex h-screen flex-col bg-background text-foreground">
      <Header
        onSettingsClick={() => setSettingsOpen(true)}
        onSwitchVault={() => {
          setSetupStep(1)
          setIsSwitching(true)
          vault.setView("setup")
        }}
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
          onCopyCode={handleCopyCode}
          onCopyPassword={handleCopyPassword}
          onRemove={handleRemove}
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
        onCredentialsChanged={creds.refresh}
        updateInfo={updateInfo}
      />
    </div>
  )
}

class ErrorBoundary extends Component<
  { children: ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Unhandled render error:", error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background p-8 text-center">
          <h1 className="text-xl font-semibold text-destructive">Something went wrong</h1>
          <p className="max-w-md text-sm text-muted-foreground">{this.state.error.message}</p>
          <button
            className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground"
            onClick={() => this.setState({ error: null })}
          >
            Try again
          </button>
        </div>
      )
    }
    return this.props.children
  }
}

export default function AppWithBoundary() {
  return (
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  )
}
