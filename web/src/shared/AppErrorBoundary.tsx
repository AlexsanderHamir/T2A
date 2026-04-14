import React, { type ErrorInfo } from "react";

const DEFAULT_FALLBACK_MESSAGE =
  "Something went wrong while rendering this page.";

type AppErrorBoundaryProps = {
  children: React.ReactNode;
  /** Bump remount keys (e.g. on `<App key={…} />`) so a soft reset can replace the failing tree without a full page reload. */
  onRecover?: () => void;
  /** User-visible headline in the fallback callout (defaults to full-app copy). */
  fallbackMessage?: string;
};

type AppErrorBoundaryState = {
  hasError: boolean;
};

export class AppErrorBoundary extends React.Component<
  AppErrorBoundaryProps,
  AppErrorBoundaryState
> {
  state: AppErrorBoundaryState = {
    hasError: false,
  };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error("[AppErrorBoundary] unhandled render error", {
      error,
      componentStack: errorInfo.componentStack,
    });
  }

  private handleSoftReset = (): void => {
    this.props.onRecover?.();
    this.setState({ hasError: false });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div className="err error-banner" role="alert" aria-live="assertive">
          <span className="error-banner__text">
            {this.props.fallbackMessage ?? DEFAULT_FALLBACK_MESSAGE}
          </span>
          <div className="task-detail-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={this.handleSoftReset}
            >
              Try again
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                window.location.reload();
              }}
            >
              Reload page
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
