import { useState } from "react";

export type RevealKind = "env" | "file";

export interface SensitiveReveals {
  /** The revealed plaintext, or undefined while the value is still masked. */
  value: (kind: RevealKind, name: string) => string | undefined;
  /** True while this one value's reveal request is in flight. */
  busy: (kind: RevealKind, name: string) => boolean;
  /** Reveals the value, or re-masks it when it is already showing. */
  toggle: (kind: RevealKind, name: string) => void;
  /** Re-masks everything — used when the draft that revealed them ends. */
  clear: () => void;
}

/**
 * Reveal state for the masked env-var and secret-file rows. Both kinds are one
 * name→plaintext map under a composite key, so the reveal/hide/in-flight rules
 * are written once instead of once per kind.
 */
export function useSensitiveReveals(
  reveal: Record<RevealKind, (name: string) => Promise<string>>,
  onError: (kind: RevealKind) => void,
): SensitiveReveals {
  const [values, setValues] = useState<Record<string, string>>({});
  const [pending, setPending] = useState<string | null>(null);

  return {
    value: (kind, name) => values[`${kind}:${name}`],
    busy: (kind, name) => pending === `${kind}:${name}`,
    clear: () => setValues({}),
    toggle: (kind, name) => {
      const id = `${kind}:${name}`;
      if (values[id] !== undefined) {
        setValues((current) =>
          Object.fromEntries(
            Object.entries(current).filter(([key]) => key !== id),
          ),
        );
        return;
      }
      setPending(id);
      void reveal[kind](name)
        .then((value) => setValues((current) => ({ ...current, [id]: value })))
        .catch(() => onError(kind))
        .finally(() => setPending(null));
    },
  };
}
