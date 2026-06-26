import type { SettingsFormState } from "../settingsForm";

export type HandleField = <K extends keyof SettingsFormState>(
  key: K,
  value: SettingsFormState[K],
) => void;
