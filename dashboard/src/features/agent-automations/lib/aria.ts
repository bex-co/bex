/** The ids a control should reference in `aria-describedby`. */
export function describedBy(
  id: string,
  { hasError, hasHelp }: { hasError: boolean; hasHelp: boolean },
): string | undefined {
  const ids = [hasError ? `${id}-error` : null, hasHelp ? `${id}-help` : null];
  const joined = ids.filter(Boolean).join(" ");
  return joined === "" ? undefined : joined;
}
