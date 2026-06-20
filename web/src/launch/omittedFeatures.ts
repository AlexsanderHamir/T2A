/**
 * Launch-time UI omissions. Backend routes and domain types stay intact;
 * the SPA hides or fixes behavior documented in docs/omitted-features.md.
 */
export const OMITTED_UI_FEATURES = {
  /** Project nav, pages, list column/filter, and create/edit project picker. */
  projects: true,
  /** Tags, milestone, and depends-on fields in the create/edit task modal. */
  tagsAndDependencies: true,
  /** Schedule for / pickup-not-before in the create/edit task modal. */
  schedule: true,
} as const;

export type OmittedUiFeature = keyof typeof OMITTED_UI_FEATURES;

export function isUiFeatureOmitted(feature: OmittedUiFeature): boolean {
  return OMITTED_UI_FEATURES[feature];
}
