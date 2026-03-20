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
