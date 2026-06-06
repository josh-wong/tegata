import { Component } from "react"
import type { ErrorInfo, ReactNode } from "react"
import i18n from "@/lib/i18n"

export class ErrorBoundary extends Component<
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
        <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background p-8 text-center">
          <h1 className="text-xl font-semibold text-destructive">{i18n.t("gui.error.somethingWentWrong")}</h1>
          <p className="max-w-md text-sm text-muted-foreground">{this.state.error.message}</p>
          <button
            className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground"
            onClick={() => this.setState({ error: null })}
          >
            {i18n.t("gui.error.tryAgain")}
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
