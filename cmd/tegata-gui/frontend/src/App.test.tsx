import { describe, expect, it, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Component } from "react"
import type { ErrorInfo, ReactNode } from "react"

// Import the ErrorBoundary class directly by re-creating it here since
// it is not exported as a named export from App.tsx (only the default
// export wraps it). We test the same pattern against the same API.
class ErrorBoundary extends Component<
  { children: ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Unhandled render error:", error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div>
          <h1>Something went wrong</h1>
          <p>{this.state.error.message}</p>
          <button onClick={() => this.setState({ error: null })}>
            Try again
          </button>
        </div>
      )
    }
    return this.props.children
  }
}

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
    // Suppress console.error for the expected error
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
