import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect, vi, beforeEach } from "vitest"
import { AuditPanel } from "./AuditPanel"
import { App } from "@/lib/wails"

vi.mock("@/lib/wails")

describe("AuditPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // GetAuditDockerPath is called automatically in useEffect when open=true.
    // Without this, auto-mock returns undefined and .then() throws.
    vi.mocked(App.GetAuditDockerPath).mockResolvedValue("")
  })

  it("renders nothing when closed", () => {
    const { container } = render(<AuditPanel open={false} onClose={() => {}} />)
    expect(container.innerHTML).toBe("")
  })

  it("renders title and buttons when open", () => {
    render(<AuditPanel open={true} onClose={() => {}} />)
    expect(screen.getByText("Audit")).toBeInTheDocument()
    expect(screen.getByText("Refresh")).toBeInTheDocument()
    expect(screen.getByText("Verify integrity")).toBeInTheDocument()
  })

  it("fetches and displays history with truncated hash", async () => {
    vi.mocked(App.GetAuditHistory).mockResolvedValue([
      { object_id: "evt-1", operation: "TOTP", label: "GitHub", label_hash: "abc123def456", timestamp: 1700000000, hash_value: "abcdef1234567890" },
    ])

    render(<AuditPanel open={true} onClose={() => {}} />)

    await waitFor(() => {
      expect(screen.getByText("1 of 1 events")).toBeInTheDocument()
      // Hash is truncated to first 10 chars + ellipsis
      expect(screen.getByText("abcdef1234…")).toBeInTheDocument()
    })
  })

  it("copies hash to clipboard on click without revealing full value", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })

    vi.mocked(App.GetAuditHistory).mockResolvedValue([
      { object_id: "evt-1", operation: "TOTP", label: "GitHub", label_hash: "abc123def456", timestamp: 1700000000, hash_value: "abcdef1234567890full" },
    ])

    render(<AuditPanel open={true} onClose={() => {}} />)

    await waitFor(() => {
      expect(screen.getByText("abcdef1234…")).toBeInTheDocument()
    })

    // Click the truncated hash — it should copy but stay truncated
    await userEvent.click(screen.getByText("abcdef1234…"))

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("abcdef1234567890full")
      expect(screen.getByText("abcdef1234…")).toBeInTheDocument()
      expect(screen.queryByText("abcdef1234567890full")).not.toBeInTheDocument()
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

  it("shows tamper detected warning with per-event detail", async () => {
    vi.mocked(App.VerifyAuditLog).mockResolvedValue({
      valid: false,
      event_count: 2,
      faults: ["tegata-abc12345-0000-0000-0000-000000000000: record hash has been altered"],
    })

    render(<AuditPanel open={true} onClose={() => {}} />)
    await userEvent.click(screen.getByText("Verify integrity"))

    await waitFor(() => {
      expect(screen.getByText(/TAMPERING DETECTED/)).toBeInTheDocument()
      expect(screen.getByText(/record hash has been altered/)).toBeInTheDocument()
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

  it("filters by operation type", async () => {
    vi.mocked(App.GetAuditHistory).mockResolvedValue([
      { object_id: "evt-1", operation: "totp", label: "GitHub", label_hash: "abc", timestamp: 1700000000, hash_value: "hash1234567890ab" },
      { object_id: "evt-2", operation: "hotp", label: "GitLab", label_hash: "def", timestamp: 1700000001, hash_value: "hash2234567890ab" },
    ])

    render(<AuditPanel open={true} onClose={() => {}} />)

    await waitFor(() => {
      expect(screen.getByText("2 of 2 events")).toBeInTheDocument()
    })

    // Filter to TOTP only
    const select = screen.getByRole("combobox")
    await userEvent.selectOptions(select, "totp")

    await waitFor(() => {
      expect(screen.getByText("1 of 2 events")).toBeInTheDocument()
    })
  })

  it("sorts by column on header click", async () => {
    vi.mocked(App.GetAuditHistory).mockResolvedValue([
      { object_id: "evt-1", operation: "totp", label: "GitHub", label_hash: "abc", timestamp: 1700000002, hash_value: "hash1234567890ab" },
      { object_id: "evt-2", operation: "hotp", label: "GitLab", label_hash: "def", timestamp: 1700000001, hash_value: "hash2234567890ab" },
    ])

    render(<AuditPanel open={true} onClose={() => {}} />)

    await waitFor(() => {
      expect(screen.getByText("2 of 2 events")).toBeInTheDocument()
    })

    // Click Operation header to sort ascending
    await userEvent.click(screen.getByText(/^Operation/))

    // After ascending sort by operation, "hotp" < "totp" so GitLab row comes first
    const rows = screen.getAllByRole("row")
    // rows[0] is the header row; rows[1] is the first data row
    expect(rows[1].textContent).toContain("hotp")
  })
})
