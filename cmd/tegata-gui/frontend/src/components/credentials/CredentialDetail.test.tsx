import { describe, expect, it, beforeEach, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { CredentialDetail } from "./CredentialDetail"
import { App } from "@/lib/wails"
import type { Credential } from "@/lib/types"

const totpCredential: Credential = {
  id: "cred-1",
  label: "GitHub",
  issuer: "GitHub Inc",
  type: "totp",
  algorithm: "SHA1",
  digits: 6,
  period: 30,
  counter: 0,
  tags: ["dev", "work"],
  notes: "",
}

const hotpCredential: Credential = {
  ...totpCredential,
  id: "cred-2",
  label: "HOTP Service",
  type: "hotp",
  counter: 5,
  tags: [],
}

describe("CredentialDetail", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(App.GenerateTOTP).mockResolvedValue({
      code: "654321",
      remaining: 20,
    })
  })

  it("shows 'Select a credential' when credential is null", () => {
    render(<CredentialDetail credential={null} onRemove={vi.fn()} />)
    expect(screen.getByText("Select a credential")).toBeInTheDocument()
  })

  it("renders label, issuer, and tags for a credential", async () => {
    render(<CredentialDetail credential={totpCredential} onRemove={vi.fn()} />)

    expect(screen.getByText("GitHub")).toBeInTheDocument()
    expect(screen.getByText("GitHub Inc")).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText("dev")).toBeInTheDocument()
      expect(screen.getByText("work")).toBeInTheDocument()
    })
  })

  it("shows Remove button that calls onRemove with credential id", async () => {
    const onRemove = vi.fn()
    const user = userEvent.setup()
    render(<CredentialDetail credential={totpCredential} onRemove={onRemove} />)

    await user.click(screen.getByText("Remove credential"))

    expect(onRemove).toHaveBeenCalledWith("cred-1")
  })

  it("TOTP type renders TOTPCountdown with code from GenerateTOTP", async () => {
    render(<CredentialDetail credential={totpCredential} onRemove={vi.fn()} />)

    await waitFor(() => {
      expect(App.GenerateTOTP).toHaveBeenCalledWith("GitHub")
      // The code "654321" is formatted as "654 321" by TOTPCountdown
      expect(screen.getByText("654 321")).toBeInTheDocument()
    })
  })

  it("HOTP type shows 'Generate code' button", () => {
    render(<CredentialDetail credential={hotpCredential} onRemove={vi.fn()} />)
    expect(screen.getByText("Generate code")).toBeInTheDocument()
  })

  it("does not render tags section when tags array is empty", () => {
    const noTagsCred: Credential = {
      ...totpCredential,
      tags: [],
    }
    render(<CredentialDetail credential={noTagsCred} onRemove={vi.fn()} />)
    expect(screen.queryByText("dev")).not.toBeInTheDocument()
  })
})
