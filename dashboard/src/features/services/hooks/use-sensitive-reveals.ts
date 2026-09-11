import { useState } from "react";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

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

type RevealStore = {
  generation: number;
  values: Record<string, string>;
  pending: string | null;
};

/**
 * Reveal state for the masked env-var and secret-file rows. Both kinds are one
 * name→plaintext map under a composite key, so the reveal/hide/in-flight rules
 * are written once instead of once per kind. Access-generation bumps and loss of
 * can_view_sensitive clear every reveal (w6/m144) by discarding a stale store
 * without an effect-driven reset.
 */
export function useSensitiveReveals(
  reveal: Record<RevealKind, (name: string) => Promise<string>>,
  onError: (kind: RevealKind) => void,
): SensitiveReveals {
  const { generation, canViewSensitive } = useCapabilities();
  const [store, setStore] = useState<RevealStore>({
    generation,
    values: {},
    pending: null,
  });

  const active =
    canViewSensitive && store.generation === generation
      ? store
      : { generation, values: {}, pending: null };

  return {
    value: (kind, name) => active.values[`${kind}:${name}`],
    busy: (kind, name) => active.pending === `${kind}:${name}`,
    clear: () =>
      setStore({ generation, values: {}, pending: null }),
    toggle: (kind, name) => {
      if (!canViewSensitive) return;
      const id = `${kind}:${name}`;
      if (active.values[id] !== undefined) {
        setStore({
          generation,
          pending: null,
          values: Object.fromEntries(
            Object.entries(active.values).filter(([key]) => key !== id),
          ),
        });
        return;
      }
      setStore({ generation, values: active.values, pending: id });
      void reveal[kind](name)
        .then((value) =>
          setStore((current) => {
            if (current.generation !== generation) {
              return { generation, values: {}, pending: null };
            }
            return {
              generation,
              pending: null,
              values: { ...current.values, [id]: value },
            };
          }),
        )
        .catch(() => {
          onError(kind);
          setStore((current) =>
            current.generation === generation
              ? { ...current, pending: null }
              : { generation, values: {}, pending: null },
          );
        });
    },
  };
}
