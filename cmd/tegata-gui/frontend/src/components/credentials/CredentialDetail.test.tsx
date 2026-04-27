import { describe, expect, it, beforeEach, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { CredentialDetail } from "./CredentialDetail"
import { App } from "@/lib/wails"
import { hashString } from "@/lib/utils"
import type { Credential } from "@/lib/types"

vi.mock("@/lib/utils", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/utils")>()
  return { ...actual, hashString: vi.fn().mockResolvedValue("test-label-hash") }
})

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

const staticCredential: Credential = {
  ...totpCredential,
  id: "cred-3",
  label: "Static Password",
  type: "static",
  tags: [],
}

const challengeCredential: Credential = {
  ...totpCredential,
  id: "cred-4",
  label: "Challenge Key",
  type: "challenge-response",
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

  it("shows Remove button that calls onRemove with credential id after confirmation", async () => {
    const onRemove = vi.fn()
    const user = userEvent.setup()
    render(<CredentialDetail credential={totpCredential} onRemove={onRemove} />)

    // Click the remove button to open the confirmation dialog
    await user.click(screen.getByText("Remove credential"))

    // Type "REMOVE" in the confirmation input
    const confirmInput = screen.getByPlaceholderText('Type "REMOVE" to confirm')
    await user.type(confirmInput, "REMOVE")

    // Click the Remove button in the dialog
    await user.click(screen.getByRole("button", { name: "Remove" }))

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

  it("HOTP type calls GenerateHOTP when Generate code is clicked", async () => {
    vi.mocked(App.GenerateHOTP).mockResolvedValue("987654")
    const user = userEvent.setup()
    render(<CredentialDetail credential={hotpCredential} onRemove={vi.fn()} />)

    await user.click(screen.getByText("Generate code"))

    await waitFor(() => {
      expect(App.GenerateHOTP).toHaveBeenCalledWith("HOTP Service")
      expect(screen.getByText("987654")).toBeInTheDocument()
    })
  })

  it("static type shows copy to clipboard button", () => {
    render(<CredentialDetail credential={staticCredential} onRemove={vi.fn()} />)
    expect(screen.getByText("Copy to clipboard")).toBeInTheDocument()
    expect(screen.getByText(/cleared automatically/)).toBeInTheDocument()
  })

  it("static type calls GetStaticPassword on copy", async () => {
    vi.mocked(App.GetStaticPassword).mockResolvedValue(undefined)
    const user = userEvent.setup()
    render(<CredentialDetail credential={staticCredential} onRemove={vi.fn()} />)

    await user.click(screen.getByText("Copy to clipboard"))

    await waitFor(() => {
      expect(App.GetStaticPassword).toHaveBeenCalledWith("Static Password")
    })
  })

  it("challenge-response type shows challenge input and Sign button", () => {
    render(<CredentialDetail credential={challengeCredential} onRemove={vi.fn()} />)
    expect(screen.getByPlaceholderText("Enter challenge text...")).toBeInTheDocument()
    expect(screen.getByText("Sign")).toBeInTheDocument()
  })

  it("challenge-response type calls SignChallenge and shows signature", async () => {
    vi.mocked(App.SignChallenge).mockResolvedValue("abcdef1234567890")
    const user = userEvent.setup()
    render(<CredentialDetail credential={challengeCredential} onRemove={vi.fn()} />)

    await user.type(screen.getByPlaceholderText("Enter challenge text..."), "test-challenge")
    await user.click(screen.getByText("Sign"))

    await waitFor(() => {
      expect(App.SignChallenge).toHaveBeenCalledWith("Challenge Key", "test-challenge")
      expect(screen.getByText("abcdef1234567890")).toBeInTheDocument()
      expect(screen.getByText("Signature")).toBeInTheDocument()
    })
  })

  it("does not render tags section when tags array is empty", () => {
    const noTagsCred: Credential = {
      ...totpCredential,
      tags: [],
    }
    render(<CredentialDetail credential={noTagsCred} onRemove={vi.fn()} />)
    expect(screen.queryByText("dev")).not.toBeInTheDocument()
  })

  it("does not render Audit section when auditEnabled is false", () => {
    render(<CredentialDetail credential={totpCredential} onRemove={vi.fn()} auditEnabled={false} />)
    expect(screen.queryByText("Recorded actions")).not.toBeInTheDocument()
  })

  it("shows Recorded actions count filtered to the selected credential", async () => {
    vi.mocked(hashString).mockResolvedValue("test-label-hash")
    vi.mocked(App.GetAuditHistory).mockResolvedValue([
      { object_id: "evt-1", operation: "totp", label_hash: "test-label-hash", timestamp: 1000, hash_value: "h1" },
      { object_id: "evt-2", operation: "totp", label_hash: "test-label-hash", timestamp: 2000, hash_value: "h2" },
      { object_id: "evt-3", operation: "totp", label_hash: "other-hash", timestamp: 3000, hash_value: "h3" },
    ])

    render(<CredentialDetail credential={totpCredential} onRemove={vi.fn()} auditEnabled={true} />)

    await waitFor(() => {
      expect(screen.getByText("Recorded actions")).toBeInTheDocument()
      expect(screen.getByText("2")).toBeInTheDocument()
    })
  })
})
