import type { Credential, VaultLocation, TOTPResult, UpdateInfo, AuditHistoryRecord, AuditVerifyResult } from "./types"

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
  IsRemovablePath(path: string): Promise<boolean>
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
  ExportVaultToFile(exportPassphrase: string): Promise<string>
  ImportVault(data: number[], importPassphrase: string): Promise<{ imported: number; skipped: number }>
  PickImportFile(): Promise<string>
  ImportVaultFromFile(path: string, importPassphrase: string): Promise<{ imported: number; skipped: number; path: string } | null>
  ChangePassphrase(current: string, newPass: string): Promise<void>
  VerifyRecoveryKey(key: string): Promise<boolean>
  GetVersion(): Promise<string>
  GetConfig(): Promise<Record<string, unknown>>
  GetIdleTimeoutSeconds(): Promise<number>
  SetIdleTimeoutSeconds(seconds: number): Promise<void>
  CheckForUpdate(): Promise<UpdateInfo | null>
  IsAuditEnabled(): Promise<boolean>
  GetAuditHistory(): Promise<AuditHistoryRecord[]>
  VerifyAuditLog(): Promise<AuditVerifyResult>
  GetAuditDockerPath(): Promise<string>
  StartAuditServer(): Promise<{ steps: string[] }>
  RestartAuditServer(): Promise<void>
  StopAuditServer(): Promise<void>
  IsAuditConfigured(): Promise<boolean>
  GetAuditAutoStart(): Promise<boolean>
  SetAuditAutoStart(enabled: boolean): Promise<void>
  EnableAudit(): Promise<void>
}

function getApp(): WailsAppBindings {
  if (window.go?.main?.App) return window.go.main.App
  throw new Error("Wails bindings not available")
}

export const App = {
  ScanForVaults: () => getApp().ScanForVaults(),
  ScanRemovableDrives: () => getApp().ScanRemovableDrives(),
  IsRemovablePath: (path: string) => getApp().IsRemovablePath(path),
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
  ExportVaultToFile: (passphrase: string) => getApp().ExportVaultToFile(passphrase),
  ImportVault: (data: number[], passphrase: string) => getApp().ImportVault(data, passphrase),
  PickImportFile: () => getApp().PickImportFile(),
  ImportVaultFromFile: (path: string, passphrase: string) => getApp().ImportVaultFromFile(path, passphrase),
  ChangePassphrase: (current: string, newPass: string) => getApp().ChangePassphrase(current, newPass),
  VerifyRecoveryKey: (key: string) => getApp().VerifyRecoveryKey(key),
  GetVersion: () => getApp().GetVersion(),
  GetConfig: () => getApp().GetConfig(),
  GetIdleTimeoutSeconds: () => getApp().GetIdleTimeoutSeconds(),
  SetIdleTimeoutSeconds: (seconds: number) => getApp().SetIdleTimeoutSeconds(seconds),
  CheckForUpdate: () => getApp().CheckForUpdate(),
  IsAuditEnabled: () => getApp().IsAuditEnabled(),
  GetAuditHistory: () => getApp().GetAuditHistory(),
  VerifyAuditLog: () => getApp().VerifyAuditLog(),
  GetAuditDockerPath: () => getApp().GetAuditDockerPath(),
  StartAuditServer: () => getApp().StartAuditServer(),
  RestartAuditServer: () => getApp().RestartAuditServer(),
  StopAuditServer: () => getApp().StopAuditServer(),
  IsAuditConfigured: () => getApp().IsAuditConfigured(),
  GetAuditAutoStart: () => getApp().GetAuditAutoStart(),
  SetAuditAutoStart: (enabled: boolean) => getApp().SetAuditAutoStart(enabled),
  EnableAudit: () => getApp().EnableAudit(),
}

export function EventsOn(event: string, callback: (...data: unknown[]) => void) {
  window.runtime?.EventsOn(event, callback)
}

export function EventsOff(event: string) {
  window.runtime?.EventsOff(event)
}
