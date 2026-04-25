import { describe, expect, it, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Sidebar } from "./Sidebar"
import type { Credential } from "@/lib/types"

const cred1: Credential = {
  id: "cred-a",
  label: "GitHub",
  issuer: "GitHub Inc",
  type: "totp",
  algorithm: "SHA1",
  digits: 6,
  period: 30,
  counter: 0,
  tags: [],
  notes: "",
}

const cred2: Credential = {
  id: "cred-b",
  label: "AWS",
  issuer: "Amazon",
  type: "totp",
  algorithm: "SHA1",
  digits: 6,
  period: 30,
  counter: 0,
  tags: [],
  notes: "",
}

function makeProps(overrides: Partial<Parameters<typeof Sidebar>[0]> = {}) {
  return {
    credentials: [cred1, cred2],
    selectedId: null,
    onSelect: vi.fn(),
    searchQuery: "",
    onSearchChange: vi.fn(),
    onAddClick: vi.fn(),
    onCopyCode: vi.fn(),
    onCopyPassword: vi.fn(),
    onRemove: vi.fn(),
    ...overrides,
  }
}

/** Right-clicks the credential button containing the given label text. */
function rightClickCred(label: string) {
  const btn = screen.getByText(label).closest("button")!
  fireEvent.contextMenu(btn)
}

describe("Sidebar bulk deletion", () => {
  it("'Remove multiple' in context menu enters selection mode", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))

    // Done button appears and type badges are hidden in selection mode
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument()
  })

  it("right-clicked credential is pre-selected when entering selection mode", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))

    // One credential selected → bulk remove button is visible
    expect(screen.getByText(/Remove 1 selected/)).toBeInTheDocument()
  })

  it("Done button exits selection mode and clears selection", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))
    await user.click(screen.getByRole("button", { name: "Cancel" }))

    expect(screen.queryByRole("button", { name: "Cancel" })).not.toBeInTheDocument()
    expect(screen.queryByText(/Remove \d+ selected/)).not.toBeInTheDocument()
  })

  it("clicking a credential in selection mode toggles its checkbox", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))

    // Select AWS as well — count goes from 1 to 2
    await user.click(screen.getByText("AWS").closest("button")!)
    expect(screen.getByText(/Remove 2 selected/)).toBeInTheDocument()

    // Deselect GitHub — count goes back to 1
    await user.click(screen.getByText("GitHub").closest("button")!)
    expect(screen.getByText(/Remove 1 selected/)).toBeInTheDocument()
  })

  it("bulk delete button is hidden when no credentials are selected", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))

    // Deselect the pre-selected credential
    await user.click(screen.getByText("GitHub").closest("button")!)

    expect(screen.queryByText(/Remove \d+ selected/)).not.toBeInTheDocument()
  })

  it("bulk delete confirmation dialog lists selected credentials", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))
    await user.click(screen.getByText(/Remove 1 selected/))

    // Dialog title and the selected credential's label should be visible
    expect(screen.getByText("Remove 1 credentials?")).toBeInTheDocument()
    expect(screen.getAllByText("GitHub").length).toBeGreaterThan(0)
  })

  it("Remove all button is disabled until REMOVE is typed", async () => {
    const user = userEvent.setup()
    render(<Sidebar {...makeProps()} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))
    await user.click(screen.getByText(/Remove 1 selected/))

    expect(screen.getByRole("button", { name: "Remove all" })).toBeDisabled()

    await user.type(screen.getByPlaceholderText('Type "REMOVE" to confirm'), "REMOVE")

    expect(screen.getByRole("button", { name: "Remove all" })).toBeEnabled()
  })

  it("confirming bulk delete calls onRemove for each selected credential", async () => {
    const onRemove = vi.fn()
    const user = userEvent.setup()
    render(<Sidebar {...makeProps({ onRemove })} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))

    // Select AWS as well
    await user.click(screen.getByText("AWS").closest("button")!)
    await user.click(screen.getByText(/Remove 2 selected/))

    await user.type(screen.getByPlaceholderText('Type "REMOVE" to confirm'), "REMOVE")
    await user.click(screen.getByRole("button", { name: "Remove all" }))

    expect(onRemove).toHaveBeenCalledWith("cred-a")
    expect(onRemove).toHaveBeenCalledWith("cred-b")
    expect(onRemove).toHaveBeenCalledTimes(2)
  })

  it("Cancel in bulk delete dialog dismisses without calling onRemove", async () => {
    const onRemove = vi.fn()
    const user = userEvent.setup()
    render(<Sidebar {...makeProps({ onRemove })} />)

    rightClickCred("GitHub")
    await user.click(screen.getByText("Remove multiple"))
    await user.click(screen.getByText(/Remove 1 selected/))
    await user.click(screen.getByRole("button", { name: "Cancel" }))

    expect(onRemove).not.toHaveBeenCalled()
    expect(screen.queryByText("Remove 1 credentials?")).not.toBeInTheDocument()
  })
})
