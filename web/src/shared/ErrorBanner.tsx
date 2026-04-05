import { useEffect, useState } from "react";

type Props = {
  message: string;
};

export function ErrorBanner({ message }: Props) {
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    setDismissed(false);
  }, [message]);

  if (dismissed) return null;

  return (
    <div
      className="err error-banner"
      role="alert"
      aria-live="assertive"
      aria-atomic="true"
    >
      <span className="error-banner__text">{message}</span>
      <button
        type="button"
        className="error-banner__dismiss"
        aria-label="Dismiss error"
        onClick={() => setDismissed(true)}
      >
        Dismiss
      </button>
    </div>
  );
}
