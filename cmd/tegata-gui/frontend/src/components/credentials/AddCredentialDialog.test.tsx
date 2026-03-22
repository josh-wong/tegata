import { describe, expect, it, beforeEach, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AddCredentialDialog } from "./AddCredentialDialog"
import { App } from "@/lib/wails"

describe("AddCredentialDialog", () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onAdded: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(App.AddCredential).mockResolvedValue("new-id")
    vi.mocked(App.AddCredentialFromURI).mockResolvedValue("new-id")
  })

  it("returns null when open is false", () => {
    const { container } = render(
      <AddCredentialDialog open={false} onClose={vi.fn()} onAdded={vi.fn()} />,
    )
    expect(container.innerHTML).toBe("")
  })

  it("renders form when open is true with Manual entry and Paste URI tabs", () => {
    render(<AddCredentialDialog {...defaultProps} />)
    expect(screen.getByText("Manual entry")).toBeInTheDocument()
    expect(screen.getByText("Paste URI")).toBeInTheDocument()
  })

  it("shows 'Label and secret are required' when submitting empty form", async () => {
    const user = userEvent.setup()
    render(<AddCredentialDialog {...defaultProps} />)

    await user.click(screen.getByText("Add"))

    expect(screen.getByText("Label and secret are required")).toBeInTheDocument()
  })

  it("shows base32 validation error for invalid secret on TOTP type", async () => {
    const user = userEvent.setup()
    render(<AddCredentialDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText("Label (required)"), "test-cred")
    // Secret input has type=password, find by placeholder
    const secretInput = screen.getByPlaceholderText("Secret (required)")
    await user.type(secretInput, "!!!")

    await user.click(screen.getByText("Add"))

    expect(
      screen.getByText(/Secret contains invalid characters/),
    ).toBeInTheDocument()
  })

  it("successful manual add calls App.AddCredential then onAdded and onClose", async () => {
    const user = userEvent.setup()
    render(<AddCredentialDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText("Label (required)"), "test-cred")
    const secretInput = screen.getByPlaceholderText("Secret (required)")
    await user.type(secretInput, "JBSWY3DPEHPK3PXP")

    await user.click(screen.getByText("Add"))

    await waitFor(() => {
      expect(App.AddCredential).toHaveBeenCalledTimes(1)
      expect(defaultProps.onAdded).toHaveBeenCalledTimes(1)
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1)
    })
  })

  it("URI tab submits via App.AddCredentialFromURI", async () => {
    const user = userEvent.setup()
    render(<AddCredentialDialog {...defaultProps} />)

    await user.click(screen.getByText("Paste URI"))

    const textarea = screen.getByPlaceholderText(/otpauth:\/\//i)
    await user.type(textarea, "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP")

    await user.click(screen.getByText("Add"))

    await waitFor(() => {
      expect(App.AddCredentialFromURI).toHaveBeenCalledWith(
        "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP",
      )
      expect(defaultProps.onAdded).toHaveBeenCalledTimes(1)
    })
  })

  it("secret input uses type='password'", () => {
    render(<AddCredentialDialog {...defaultProps} />)
    const secretInput = screen.getByPlaceholderText("Secret (required)")
    expect(secretInput).toHaveAttribute("type", "password")
  })

  it("Cancel button calls onClose", async () => {
    const user = userEvent.setup()
    render(<AddCredentialDialog {...defaultProps} />)

    // Dialog renders both a header close and a footer Cancel button
    const cancelButtons = screen.getAllByText("Cancel")
    await user.click(cancelButtons[0])

    expect(defaultProps.onClose).toHaveBeenCalledTimes(1)
  })
})
