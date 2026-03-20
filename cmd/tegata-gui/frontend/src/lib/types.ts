export type CredentialType = "totp" | "hotp" | "static" | "cr"

export interface Credential {
  id: string
  label: string
  issuer: string
  type: CredentialType
  algorithm: string
  digits: number
  period: number
  counter: number
  tags: string[]
  notes: string
}

export type AppView = "loading" | "setup" | "unlock" | "main"

export interface VaultLocation {
  path: string
  driveName: string
}

export interface TOTPResult {
  Code: string
  Remaining: number
}

export interface UpdateInfo {
  Version: string
  URL: string
  Notes: string
}
