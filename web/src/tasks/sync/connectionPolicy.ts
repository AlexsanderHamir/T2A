// Module-level flag mirroring useTaskEventStream connection state.
// QueryClient refetchOnWindowFocus reads this outside the React render path.
let sseLiveForQueries = false;

export function setSseLiveForQueries(connected: boolean): void {
  sseLiveForQueries = connected;
}

export function isSseLiveForQueries(): boolean {
  return sseLiveForQueries;
}
