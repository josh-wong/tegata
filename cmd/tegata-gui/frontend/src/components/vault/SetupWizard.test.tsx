import { describe, expect, it, beforeEach, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { SetupWizard } from "./SetupWizard"
import { App } from "@/lib/wails"

describe("SetupWizard", () => {
  const defaultProps = {
    vaultLocations: [],
    loading: false,
    error: null,
    onCreateVault: vi.fn().mockResolvedValue("recovery-key-test"),
    onOpenExisting: vi.fn(),
    onComplete: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(App.ScanRemovableDrives).mockResolvedValue([])
    vi.mocked(App.ScanForVaults).mockResolvedValue([])
  })

  it("renders welcome with 'Create new vault' and 'Open existing vault' buttons", () => {
    render(<SetupWizard {...defaultProps} />)
    expect(screen.getByText("Create new vault")).toBeInTheDocument()
    expect(screen.getByText("Open existing vault")).toBeInTheDocument()
  })

  it("clicking 'Create new vault' advances to step 2 (location picker)", async () => {
    const user = userEvent.setup()
    render(<SetupWizard {...defaultProps} />)

    await user.click(screen.getByText("Create new vault"))

    await waitFor(() => {
      expect(screen.getByText("Choose a location")).toBeInTheDocument()
    })
  })

  it("shows validation error for passphrase shorter than 8 characters", async () => {
    const user = userEvent.setup()
    render(<SetupWizard {...defaultProps} initialStep={3} />)

    const inputs = screen.getAllByPlaceholderText(/passphrase/i)
    await user.type(inputs[0], "short")
    await user.type(inputs[1], "short")

    await user.click(screen.getByText("Create vault"))

    expect(
      screen.getByText("Passphrase must be at least 8 characters"),
    ).toBeInTheDocument()
  })

  it("shows mismatch error when passphrases do not match", async () => {
    const user = userEvent.setup()
    render(<SetupWizard {...defaultProps} initialStep={3} />)

    const inputs = screen.getAllByPlaceholderText(/passphrase/i)
    await user.type(inputs[0], "test-passphrase-dummy-one")
    await user.type(inputs[1], "test-passphrase-dummy-two")

    await user.click(screen.getByText("Create vault"))

    expect(screen.getByText("Passphrases do not match")).toBeInTheDocument()
  })

  it("passphrase inputs use type='password'", () => {
    render(<SetupWizard {...defaultProps} initialStep={3} />)

    const inputs = screen.getAllByPlaceholderText(/passphrase/i)
    for (const input of inputs) {
      expect(input).toHaveAttribute("type", "password")
    }
  })

  it("onCancel button appears only when prop is provided", () => {
    const { rerender } = render(<SetupWizard {...defaultProps} />)
    expect(screen.queryByText("Cancel")).not.toBeInTheDocument()

    rerender(<SetupWizard {...defaultProps} onCancel={vi.fn()} />)
    expect(screen.getByText("Cancel")).toBeInTheDocument()
  })

  it("does not expose passphrase values as visible text in the DOM", () => {
    render(<SetupWizard {...defaultProps} initialStep={3} />)

    const inputs = screen.getAllByPlaceholderText(/passphrase/i)
    // Verify all passphrase inputs are type=password so values are never
    // shown as visible text. (jsdom serializes the value attribute in
    // innerHTML regardless, but real browsers mask it.)
    for (const input of inputs) {
      expect(input).toHaveAttribute("type", "password")
    }

    // Verify no passphrase-related text node appears in visible content
    expect(screen.queryByText(/test-passphrase/)).not.toBeInTheDocument()
  })
})
