import type { TimezoneSelectOption } from "@/shared/time/appTimezone";
import { formatTimezoneMenuLabel } from "@/shared/time/appTimezone";
import { TimezoneCombobox } from "../TimezoneCombobox";
import type { SettingsFormState } from "../settingsForm";
import { SECTION_IDS } from "./sectionIds";
import { SectionCard } from "./settingsSectionLayout";
import type { HandleField } from "./settingsSectionTypes";

export function DisplaySettingsSection({
  form,
  browserTz,
  options,
  showCustomTz,
  onField,
}: {
  form: SettingsFormState;
  browserTz: string;
  options: TimezoneSelectOption[];
  showCustomTz: boolean;
  onField: HandleField;
}) {
  const customValue = form.displayTimezone.trim();
  return (
    <SectionCard id={SECTION_IDS.display} title="Display">
      <label className="settings-field">
        <span className="settings-field-label">Timezone</span>
        <TimezoneCombobox
          testId="settings-display-timezone-select"
          value={form.displayTimezone}
          onChange={(v) => onField("displayTimezone", v)}
          browserTz={browserTz}
          options={options}
          customSaved={
            showCustomTz
              ? {
                  value: customValue,
                  label: `${formatTimezoneMenuLabel(customValue)} (saved — not in list)`,
                }
              : null
          }
        />
      </label>
      <p className="settings-field-help">
        Used for every operator-facing timestamp. Storage stays in UTC.
      </p>
    </SectionCard>
  );
}
