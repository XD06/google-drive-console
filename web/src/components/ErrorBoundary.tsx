import { Component, type ErrorInfo, type ReactNode } from "react";

export type ErrorBoundaryProps = {
  children: ReactNode;
  /** Optional custom fallback; when omitted a themed recovery screen is shown. */
  fallback?: ReactNode;
};

type ErrorBoundaryState = {
  hasError: boolean;
};

/**
 * Top-level error boundary. Without it, any exception thrown while rendering
 * (e.g. a NaN in the lightbox zoom math) unmounts the whole tree and leaves a
 * blank white page. Catching it here keeps the app recoverable with a reload.
 *
 * This is the one place React still requires a class component — hooks cannot
 * implement getDerivedStateFromError / componentDidCatch.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Surface the crash in the console for debugging; the UI only shows a
    // friendly recovery prompt (never the raw stack).
    console.error("Unhandled render error:", error, info.componentStack);
  }

  private handleReload = () => {
    window.location.reload();
  };

  render() {
    if (!this.state.hasError) return this.props.children;
    if (this.props.fallback) return this.props.fallback;
    return (
      <div className="boot" role="alert">
        <div style={{ textAlign: "center", maxWidth: 360, padding: "0 24px" }}>
          <p style={{ color: "var(--text)", fontWeight: 600, margin: "0 0 6px" }}>
            Something went wrong
          </p>
          <p style={{ margin: "0 0 16px" }}>
            The app hit an unexpected error. Reloading usually fixes it.
          </p>
          <button type="button" className="btn btn-primary" onClick={this.handleReload}>
            Reload
          </button>
        </div>
      </div>
    );
  }
}
