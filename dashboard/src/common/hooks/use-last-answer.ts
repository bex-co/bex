import { useState } from "react";

/**
 * The current answer for `key`, or — while a refresh of it is pending or has
 * failed — the last answer that key gave. Never another key's: a resource or
 * label change starts with nothing. Adjusted during render, React's pattern
 * for state derived from props; an identical answer does not churn state.
 */
export function useLastAnswer(
  key: string,
  answered: string[] | null,
): string[] | null {
  const [last, setLast] = useState<{ key: string; values: string[] } | null>(
    null,
  );
  const kept = last?.key === key ? last.values : null;
  if (answered && !(kept && sameValues(kept, answered))) {
    setLast({ key, values: answered });
  }
  if (answered) return kept && sameValues(kept, answered) ? kept : answered;
  return kept;
}

function sameValues(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}
