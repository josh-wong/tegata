import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect, vi, beforeEach } from "vitest"
import { AuditPanel } from "./AuditPanel"
import { App } from "@/lib/wails"

vi.mock("@/lib/wails")

describe("AuditPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("renders nothing when closed", () => {
    const { container } = render(<AuditPanel open={false} onClose={() => {}} />)
    expect(container.innerHTML).toBe("")
  })

  it("renders title and buttons when open", () => {
    render(<AuditPanel open={true} onClose={() => {}} />)
    expect(screen.getByText("Audit")).toBeInTheDocument()
    expect(screen.getByText("View history")).toBeInTheDocument()
    expect(screen.getByText("Verify integrity")).toBeInTheDocument()
  })

  it("fetches and displays history", async () => {
    vi.mocked(App.GetAuditHistory).mockResolvedValue([
      { hash_value: "abcd1234", version: 0 },
    ])

    render(<AuditPanel open={true} onClose={() => {}} />)
    await userEvent.click(screen.getByText("View history"))

    await waitFor(() => {
      expect(screen.getByText("1 events")).toBeInTheDocument()
      expect(screen.getByText("abcd1234")).toBeInTheDocument()
    })
  })

  it("shows verified result", async () => {
    vi.mocked(App.VerifyAuditLog).mockResolvedValue({
      valid: true,
      event_count: 3,
    })

    render(<AuditPanel open={true} onClose={() => {}} />)
    await userEvent.click(screen.getByText("Verify integrity"))

    await waitFor(() => {
      expect(screen.getByText(/integrity verified/i)).toBeInTheDocument()
      expect(screen.getByText(/3 events/i)).toBeInTheDocument()
    })
  })

  it("shows tamper detected warning", async () => {
    vi.mocked(App.VerifyAuditLog).mockResolvedValue({
      valid: false,
      event_count: 2,
      error_detail: "hash mismatch at version 1",
    })

    render(<AuditPanel open={true} onClose={() => {}} />)
    await userEvent.click(screen.getByText("Verify integrity"))

    await waitFor(() => {
      expect(screen.getByText(/TAMPER DETECTED/)).toBeInTheDocument()
      expect(screen.getByText(/hash mismatch/)).toBeInTheDocument()
    })
  })

  it("shows nothing to verify for empty events", async () => {
    vi.mocked(App.VerifyAuditLog).mockResolvedValue({
      valid: true,
      event_count: 0,
    })

    render(<AuditPanel open={true} onClose={() => {}} />)
    await userEvent.click(screen.getByText("Verify integrity"))

    await waitFor(() => {
      expect(screen.getByText(/Nothing to verify/)).toBeInTheDocument()
    })
  })
})
