export type CredentialType = "totp" | "hotp" | "static" | "challenge-response"

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
  created_at: string
  modified_at: string
}

export type AppView = "loading" | "setup" | "unlock" | "main"

export interface VaultLocation {
  path: string
  driveName: string
}

export interface TOTPResult {
  code: string
  remaining: number
}

export interface UpdateInfo {
  version: string
  url: string
  notes: string
}

export interface AuditHistoryRecord {
  object_id: string
  operation: string
  label_hash: string
  timestamp: number
  hash_value: string
}

export interface AuditVerifyResult {
  valid: boolean
  event_count: number
  skipped?: number
  faults?: string[]
}
