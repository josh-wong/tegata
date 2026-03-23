import { describe, expect, it, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { ErrorBoundary } from "@/components/shared/ErrorBoundary"

function GoodChild() {
  return <div>Everything is fine</div>
}

let shouldThrow = false

function ThrowingChild() {
  if (shouldThrow) throw new Error("Test render error")
  return <div>Recovered successfully</div>
}

describe("ErrorBoundary", () => {
  beforeEach(() => {
    shouldThrow = false
  })

  it("renders children normally when no error", () => {
    render(
      <ErrorBoundary>
        <GoodChild />
      </ErrorBoundary>,
    )
    expect(screen.getByText("Everything is fine")).toBeInTheDocument()
  })

  it("shows 'Something went wrong' and error message when child throws", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {})

    shouldThrow = true
    render(
      <ErrorBoundary>
        <ThrowingChild />
      </ErrorBoundary>,
    )

    expect(screen.getByText("Something went wrong")).toBeInTheDocument()
    expect(screen.getByText("Test render error")).toBeInTheDocument()

    spy.mockRestore()
  })

  it("'Try again' button recovers by re-rendering children", async () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {})
    const user = userEvent.setup()

    shouldThrow = true
    render(
      <ErrorBoundary>
        <ThrowingChild />
      </ErrorBoundary>,
    )

    expect(screen.getByText("Something went wrong")).toBeInTheDocument()

    // Now fix the child so it won't throw
    shouldThrow = false

    await user.click(screen.getByText("Try again"))

    expect(screen.getByText("Recovered successfully")).toBeInTheDocument()
    expect(screen.queryByText("Something went wrong")).not.toBeInTheDocument()

    spy.mockRestore()
  })
})
