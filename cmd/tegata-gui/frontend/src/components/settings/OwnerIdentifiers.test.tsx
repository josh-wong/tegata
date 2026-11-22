import { describe, expect, it, beforeEach, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { OwnerIdentifiers } from "./OwnerIdentifiers"
import { App } from "@/lib/wails"
import type { OwnerInfo } from "@/lib/types"

const emptyOwner: OwnerInfo = { employeeIdHash: "", emailHash: "", hashScheme: "", updatedAt: "", saltConfigured: false }
const setOwner: OwnerInfo = { employeeIdHash: "a1b2c3d4e5f6", emailHash: "0f9e8d7c6b5a", hashScheme: "sha256", updatedAt: "2026-11-22T00:00:00Z", saltConfigured: false }

describe("OwnerIdentifiers", () => {
  beforeEach(() => {
    vi.mocked(App.GetOwner).mockResolvedValue(emptyOwner)
    vi.mocked(App.SetOwner).mockResolvedValue(setOwner)
  })

  it("shows not set when no identifiers are stored", async () => {
    render(<OwnerIdentifiers />)
    await waitFor(() => expect(App.GetOwner).toHaveBeenCalled())
    expect(screen.getAllByText("Not set")).toHaveLength(2)
  })

  it("shows only short hash prefixes for stored identifiers", async () => {
    vi.mocked(App.GetOwner).mockResolvedValue(setOwner)
    render(<OwnerIdentifiers />)
    expect(await screen.findByText("Current: a1b2c3d4e5f6")).toBeInTheDocument()
    expect(screen.getByText("Current: 0f9e8d7c6b5a")).toBeInTheDocument()
  })

  it("sends trimmed values and clears the form after saving", async () => {
    const user = userEvent.setup()
    render(<OwnerIdentifiers />)
    await user.click(await screen.findByRole("button", { name: "Edit identifiers" }))
    await user.type(screen.getByLabelText("Employee ID (recommended)"), "  acme-emp-1  ")
    await user.click(screen.getByRole("button", { name: "Save" }))

    expect(App.SetOwner).toHaveBeenCalledWith("acme-emp-1", "", false, false)
    expect(await screen.findByText("Owner identifiers saved.")).toBeInTheDocument()
    expect(screen.queryByLabelText("Employee ID (recommended)")).not.toBeInTheDocument()
  })

  it("shows a localized message when the organization salt changed", async () => {
    vi.mocked(App.SetOwner).mockRejectedValueOnce("owner-salt-changed")
    const user = userEvent.setup()
    render(<OwnerIdentifiers />)
    await user.click(await screen.findByRole("button", { name: "Edit identifiers" }))
    await user.type(screen.getByLabelText("Email address"), "alex@example.com")
    await user.click(screen.getByRole("button", { name: "Save" }))

    expect(await screen.findByRole("alert")).toHaveTextContent(/salt has changed/)
    expect(screen.getByLabelText("Email address")).toHaveValue("")
  })

  it("gives each clear checkbox a distinct accessible name", async () => {
    vi.mocked(App.GetOwner).mockResolvedValue(setOwner)
    const user = userEvent.setup()
    render(<OwnerIdentifiers />)
    await user.click(await screen.findByRole("button", { name: "Edit identifiers" }))
    await user.click(screen.getByRole("checkbox", { name: "Clear email address" }))
    expect(screen.getByLabelText("Email address")).toBeDisabled()
    expect(screen.getByRole("checkbox", { name: "Clear employee ID" })).not.toBeChecked()
    await user.click(screen.getByRole("button", { name: "Save" }))
    expect(App.SetOwner).toHaveBeenLastCalledWith("", "", false, true)
  })
})
