export function SettingsLoadingState({
  error,
  onRetry,
}: {
  error: Error | null;
  onRetry: () => void;
}) {
  return (
    <section className="settings-page" aria-busy="true">
      <h2 className="settings-page-title">Settings</h2>
      <p>{error ? `Error: ${error.message}` : "Loading settings…"}</p>
      {error ? (
        <button type="button" onClick={onRetry}>
          Retry
        </button>
      ) : null}
    </section>
  );
}
