export function taskCreateModalDmapReady(
  dmapMode: boolean,
  dmapCommitLimit: string,
  dmapDomain: string,
): boolean {
  if (!dmapMode) return true;
  const parsedCommitLimit = Number.parseInt(dmapCommitLimit, 10);
  const dmapCommitValid =
    Number.isInteger(parsedCommitLimit) && parsedCommitLimit > 0;
  const dmapDomainValid = dmapDomain.trim().length > 0;
  return dmapCommitValid && dmapDomainValid;
}
