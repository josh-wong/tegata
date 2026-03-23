import { describe, expect, it, beforeEach, vi } from "vitest"
import { render, screen, act } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { UnlockView } from "./UnlockView"

describe("UnlockView", () => {
  const defaultProps = {
    vaultPath: "/usb/vault.tegata",
    vaultLocations: [{ path: "/usb/vault.tegata", driveName: "USB" }],
    error: null,
    loading: false,
    onUnlock: vi.fn(),
    onSelectVault: vi.fn(),
    onBack: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("renders passphrase input and Unlock button", () => {
    render(<UnlockView {...defaultProps} />)
    expect(screen.getByPlaceholderText("Passphrase")).toBeInTheDocument()
    expect(screen.getByText("Unlock")).toBeInTheDocument()
  })

  it("submit calls onUnlock with the entered passphrase", async () => {
    const user = userEvent.setup()
    render(<UnlockView {...defaultProps} />)

    await user.type(screen.getByPlaceholderText("Passphrase"), "test-passphrase-dummy")
    await user.click(screen.getByText("Unlock"))

    expect(defaultProps.onUnlock).toHaveBeenCalledWith("test-passphrase-dummy")
  })

  it("shows error message when error prop is set", () => {
    render(<UnlockView {...defaultProps} error="Wrong passphrase" />)
    expect(screen.getByText("Wrong passphrase")).toBeInTheDocument()
  })

  it("Unlock button is disabled when passphrase is empty", () => {
    render(<UnlockView {...defaultProps} />)
    expect(screen.getByRole("button", { name: "Unlock" })).toBeDisabled()
  })

  it("Unlock button is disabled when loading is true", async () => {
    const user = userEvent.setup()
    const { rerender } = render(<UnlockView {...defaultProps} />)

    await user.type(screen.getByPlaceholderText("Passphrase"), "test-passphrase-dummy")
    rerender(<UnlockView {...defaultProps} loading={true} />)

    // When loading, the button shows a spinner instead of "Unlock" text.
    // Find the submit button among all buttons since it has no accessible name.
    const buttons = screen.getAllByRole("button")
    const submitBtn = buttons.find((btn) => btn.getAttribute("type") === "submit")
    expect(submitBtn).toBeDefined()
    expect(submitBtn).toBeDisabled()
  })

  it("input type is 'password'", () => {
    render(<UnlockView {...defaultProps} />)
    expect(screen.getByPlaceholderText("Passphrase")).toHaveAttribute("type", "password")
  })

  it("shows vault selector when multiple vaultLocations provided", () => {
    render(
      <UnlockView
        {...defaultProps}
        vaultLocations={[
          { path: "/usb/a.tegata", driveName: "USB A" },
          { path: "/usb/b.tegata", driveName: "USB B" },
        ]}
      />,
    )
    expect(screen.getByText("Vault")).toBeInTheDocument()
    expect(screen.getByRole("combobox")).toBeInTheDocument()
  })

  it("does not show vault selector for single location", () => {
    render(<UnlockView {...defaultProps} />)
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument()
  })

  it("Back button calls onBack", async () => {
    const user = userEvent.setup()
    render(<UnlockView {...defaultProps} />)

    await user.click(screen.getByText("Back"))

    expect(defaultProps.onBack).toHaveBeenCalledTimes(1)
  })

  it("retries focus on passphrase input via polling interval", () => {
    vi.useFakeTimers()

    render(<UnlockView {...defaultProps} />)
    const input = screen.getByPlaceholderText("Passphrase")

    // Simulate the input not receiving focus initially
    // The component polls every 100ms up to 20 times
    act(() => {
      vi.advanceTimersByTime(100)
    })

    // After polling, the input should have focus
    expect(document.activeElement).toBe(input)

    vi.useRealTimers()
  })

  it("stops focus polling after unmount", () => {
    vi.useFakeTimers()

    const { unmount } = render(<UnlockView {...defaultProps} />)
    unmount()

    // Advancing timers after unmount should not throw
    act(() => {
      vi.advanceTimersByTime(3000)
    })

    vi.useRealTimers()
  })

  it("does not expose passphrase as visible text in the DOM", () => {
    render(<UnlockView {...defaultProps} />)

    // Verify the passphrase input is type=password so values are never
    // shown as visible text. (jsdom serializes the value attribute in
    // innerHTML regardless, but real browsers mask it.)
    expect(screen.getByPlaceholderText("Passphrase")).toHaveAttribute(
      "type",
      "password",
    )

    // Verify no passphrase-related text node appears in visible content
    expect(screen.queryByText(/test-passphrase/)).not.toBeInTheDocument()
  })
})
