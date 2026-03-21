import type { Credential, VaultLocation, TOTPResult, UpdateInfo } from "./types"

// Wails runtime bindings facade. At build time with `wails build`, the real
// bindings are injected. This module provides typed stubs so the frontend
// type-checks and builds standalone with Vite.

declare global {
  interface Window {
    go?: {
      main: {
        App: WailsAppBindings
      }
    }
    runtime?: {
      EventsOn: (event: string, callback: (...data: unknown[]) => void) => void
      EventsOff: (event: string) => void
    }
  }
}

interface WailsAppBindings {
  ScanForVaults(): Promise<VaultLocation[]>
  ScanRemovableDrives(): Promise<VaultLocation[]>
  CreateVault(path: string, passphrase: string): Promise<string>
  UnlockVault(path: string, passphrase: string): Promise<void>
  LockVault(): Promise<void>
  ListCredentials(): Promise<Credential[]>
  GetCredential(label: string): Promise<Credential>
  AddCredential(
    label: string,
    issuer: string,
    credType: string,
    secret: string,
    algorithm: string,
    digits: number,
    period: number,
    tags: string[],
  ): Promise<string>
  AddCredentialFromURI(uri: string): Promise<string>
  RemoveCredential(id: string): Promise<void>
  GenerateTOTP(label: string): Promise<TOTPResult>
  GenerateHOTP(label: string): Promise<string>
  GetStaticPassword(label: string): Promise<void>
  SignChallenge(label: string, challenge: string): Promise<string>
  ExportVault(exportPassphrase: string): Promise<number[]>
  ImportVault(data: number[], importPassphrase: string): Promise<{ imported: number; skipped: number }>
  ChangePassphrase(current: string, newPass: string): Promise<void>
  VerifyRecoveryKey(key: string): Promise<boolean>
  GetConfig(): Promise<Record<string, unknown>>
  CheckForUpdate(): Promise<UpdateInfo | null>
}

function getApp(): WailsAppBindings {
  if (window.go?.main?.App) return window.go.main.App
  throw new Error("Wails bindings not available")
}

export const App = {
  ScanForVaults: () => getApp().ScanForVaults(),
  ScanRemovableDrives: () => getApp().ScanRemovableDrives(),
  CreateVault: (path: string, passphrase: string) => getApp().CreateVault(path, passphrase),
  UnlockVault: (path: string, passphrase: string) => getApp().UnlockVault(path, passphrase),
  LockVault: () => getApp().LockVault(),
  ListCredentials: () => getApp().ListCredentials(),
  GetCredential: (label: string) => getApp().GetCredential(label),
  AddCredential: (...args: Parameters<WailsAppBindings["AddCredential"]>) => getApp().AddCredential(...args),
  AddCredentialFromURI: (uri: string) => getApp().AddCredentialFromURI(uri),
  RemoveCredential: (id: string) => getApp().RemoveCredential(id),
  GenerateTOTP: (label: string) => getApp().GenerateTOTP(label),
  GenerateHOTP: (label: string) => getApp().GenerateHOTP(label),
  GetStaticPassword: (label: string) => getApp().GetStaticPassword(label),
  SignChallenge: (label: string, challenge: string) => getApp().SignChallenge(label, challenge),
  ExportVault: (passphrase: string) => getApp().ExportVault(passphrase),
  ImportVault: (data: number[], passphrase: string) => getApp().ImportVault(data, passphrase),
  ChangePassphrase: (current: string, newPass: string) => getApp().ChangePassphrase(current, newPass),
  VerifyRecoveryKey: (key: string) => getApp().VerifyRecoveryKey(key),
  GetConfig: () => getApp().GetConfig(),
  CheckForUpdate: () => getApp().CheckForUpdate(),
}

export function EventsOn(event: string, callback: (...data: unknown[]) => void) {
  window.runtime?.EventsOn(event, callback)
}

export function EventsOff(event: string) {
  window.runtime?.EventsOff(event)
}
