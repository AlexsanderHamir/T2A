export function SettingsHeader({
  lastUpdated,
  lastUpdatedFormatted,
}: {
  lastUpdated: string;
  lastUpdatedFormatted: string;
}) {
  return (
    <header className="settings-page-header">
      <div className="settings-page-heading">
        <h2 className="settings-page-title">Settings</h2>
        <p className="settings-page-subtitle">
          Runtime configuration for this installation.
        </p>
      </div>
      {lastUpdated ? (
        <span
          className="settings-page-saved-chip"
          data-testid="settings-last-updated"
        >
          <span className="settings-page-saved-chip-label">Last saved</span>
          <time className="settings-page-saved-chip-time" dateTime={lastUpdated}>
            {lastUpdatedFormatted || lastUpdated}
          </time>
        </span>
      ) : null}
    </header>
  );
}
