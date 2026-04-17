import type { ErrorInfo, ReactNode } from "react"
import { Component } from "react"

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
          <h1 className="text-xl font-semibold">页面出现异常</h1>
          <p className="max-w-md text-sm text-muted-foreground">
            当前页面发生了未处理错误。请刷新后重试，如果问题持续请联系平台管理员。
          </p>
          <button
            type="button"
            onClick={() => this.reload()}
            className="inline-flex items-center rounded-md border px-3 py-1.5 text-sm transition hover:bg-accent"
          >
            刷新页面
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
