import type { ErrorInfo, ReactNode } from "react"
import { Component } from "react"

import i18n from "@/lib/i18n"

type AppErrorBoundaryProps = {
  children: ReactNode
}

type AppErrorBoundaryState = {
  hasError: boolean
}

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = {
    hasError: false,
  }

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("unhandled_ui_error", {
      message: error.message,
      stack: error.stack,
      componentStack: info.componentStack,
    })
  }

  private reload(): void {
    window.location.reload()
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-background px-6 text-center">
          <h1 className="text-xl font-semibold">
            {i18n.t("appErrorBoundary.title", { defaultValue: "Something went wrong" })}
          </h1>
          <p className="max-w-md text-sm text-muted-foreground">
            {i18n.t("appErrorBoundary.description", {
              defaultValue:
                "An unhandled error occurred on this page. Refresh and try again. If the issue persists, contact your platform administrator.",
            })}
          </p>
          <button
            type="button"
            onClick={() => this.reload()}
            className="inline-flex items-center rounded-md border px-3 py-1.5 text-sm transition hover:bg-accent"
          >
            {i18n.t("appErrorBoundary.reload", { defaultValue: "Reload page" })}
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
