import { createContext, useContext, useEffect, useId } from "react";
import type { Translate } from "@/common/hooks/use-translations";
import { conjunctionList } from "@/common/lib/format/language";

/**
 * Render's unsaved-changes warning (w3/118; `useUnsavedChangesWarning`, whose
 * provider mounts at the root of Render's dashboard). An edited settings row
 * registers while it is open with a change it has not saved and is not
 * saving; while any row is registered, `UnsavedChangesProvider` holds a
 * navigation to another path with **Discard unsaved changes?** and asks
 * through the browser's `beforeunload` on a reload or a closed tab.
 */
export interface UnsavedField {
  /** The row's label, as the dialog names it ("the {label} field"). */
  label: string;
  /** The row's control, which Keep editing focuses. */
  target: () => HTMLElement | null;
}

export interface UnsavedChangesRegistry {
  register: (id: string, field: UnsavedField) => void;
  unregister: (id: string) => void;
}

export const UnsavedChangesContext =
  createContext<UnsavedChangesRegistry | null>(null);

/**
 * Registers this row while `when` holds. Outside the provider (a component
 * test that renders a row alone) it does nothing, as Render's hook does.
 * `target` must be stable (a `useCallback` over the row's refs).
 */
export function useUnsavedChangesWarning(
  when: boolean,
  label: string,
  target: () => HTMLElement | null,
) {
  const registry = useContext(UnsavedChangesContext);
  const id = useId();
  useEffect(() => {
    if (!registry || !when) return;
    registry.register(id, { label, target });
    return () => registry.unregister(id);
  }, [registry, when, id, label, target]);
}

/**
 * The edited fields as the dialog names them: Render's "the A field", "the A
 * and B fields", "the A, B, and C fields" in English, and each language's own
 * article, noun and list join elsewhere.
 */
export function editedFieldsPhrase(
  t: Translate,
  language: string | undefined,
  labels: readonly string[],
): string {
  return t("common.unsavedChangesFields", {
    count: labels.length,
    list: conjunctionList(language).format(labels),
  });
}
