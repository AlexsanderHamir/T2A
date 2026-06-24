/**
 * Launch-time UI omissions. Backend routes and domain types stay intact;
 * the SPA hides or fixes behavior documented in docs/omitted-features.md.
 */
export const OMITTED_UI_FEATURES = {
  /** Project nav, pages, list column/filter, and create/edit project picker. */
  projects: false,
  /** Tags, milestone, depends-on fields, and task detail dependencies panel. */
  tagsAndDependencies: true,
  /** Schedule for / pickup-not-before in the create/edit task modal. */
  schedule: true,
  /** Task detail release gate panel and operator gate actions. */
  releaseGates: true,
} as const;

export type OmittedUiFeature = keyof typeof OMITTED_UI_FEATURES;

export function isUiFeatureOmitted(feature: OmittedUiFeature): boolean {
  return OMITTED_UI_FEATURES[feature];
}
