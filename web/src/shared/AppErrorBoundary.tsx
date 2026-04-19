import React, { type ErrorInfo } from "react";

const DEFAULT_FALLBACK_MESSAGE =
  "Something went wrong while rendering this page.";

type AppErrorBoundaryProps = {
  children: React.ReactNode;
  /** Bump remount keys (e.g. on `<App key={…} />`) so a soft reset can replace the failing tree without a full page reload. */
  onRecover?: () => void;
  /** User-visible headline in the fallback callout (defaults to full-app copy). */
  fallbackMessage?: string;
  /** `componentDidCatch` log prefix; default `app-root` (full SPA shell in `main.tsx`). */
  variant?: "app-root" | "route-outlet";
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
    const scope = this.props.variant ?? "app-root";
    console.error(`[AppErrorBoundary:${scope}] unhandled render error`, {
      error,
      componentStack: errorInfo.componentStack,
    });
  }

  private handleSoftReset = (): void => {
    this.props.onRecover?.();
    this.setState({ hasError: false });
  };

  private handleGoHome = (): void => {
    // Hard navigation rather than react-router `navigate("/")`:
    // AppErrorBoundary is a class component (no hooks) and the
    // failed subtree may have left non-error state in a wedged
    // condition. A full document load to "/" guarantees a clean
    // mount with no leaked closures, mirroring the safety contract
    // of the Reload-page button below.
    window.location.assign("/");
  };

  render() {
    if (this.state.hasError) {
      const message = this.props.fallbackMessage ?? DEFAULT_FALLBACK_MESSAGE;
      return (
        <div
          className="app-error-fallback"
          role="alert"
          aria-live="assertive"
        >
          <div className="app-error-fallback__card">
            <div className="app-error-fallback__icon-wrap" aria-hidden="true">
              <AppErrorBoundaryGlyph />
            </div>
            <h1 className="app-error-fallback__title">Something went wrong</h1>
            <p className="app-error-fallback__description">{message}</p>
            <div className="app-error-fallback__actions">
              <button
                type="button"
                className="app-error-fallback__cta"
                onClick={this.handleSoftReset}
              >
                Try again
              </button>
              <button
                type="button"
                className="app-error-fallback__secondary"
                onClick={this.handleGoHome}
              >
                Go back
              </button>
              <button
                type="button"
                className="app-error-fallback__secondary"
                onClick={() => {
                  window.location.reload();
                }}
              >
                Reload page
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

function AppErrorBoundaryGlyph() {
  return (
    <svg
      className="app-error-fallback__glyph"
      width={44}
      height={44}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <circle
        cx={24}
        cy={24}
        r={18}
        stroke="currentColor"
        strokeWidth={1.75}
        opacity={0.55}
      />
      <path
        d="M24 16v9"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
      />
      <circle cx={24} cy={31} r={1.4} fill="currentColor" />
    </svg>
  );
}
