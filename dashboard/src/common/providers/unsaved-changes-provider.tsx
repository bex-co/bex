import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useBlocker, type ShouldBlockFn } from "@tanstack/react-router";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  aroundPlaceholder,
  PLACEHOLDER_SENTINEL,
} from "@/common/lib/i18n-placeholder";
import {
  editedFieldsPhrase,
  UnsavedChangesContext,
  type UnsavedChangesRegistry,
  type UnsavedField,
} from "./unsaved-changes-context";

/**
 * Render's root unsaved-changes warning (w3/118, Render's
 * `useUnsavedChangesWarning` provider). While an edited row is registered
 * (`useUnsavedChangesWarning`), a client navigation to another path is held
 * by **Discard unsaved changes?**, naming the edited fields as Render joins
 * them ("the A field", "the A and B fields", "the A, B, and C fields").
 * **Keep editing** (or Escape) stays and focuses the first edited field;
 * **Discard** goes on, and each row's unmount drops its edit. A navigation
 * that keeps the path (a query or hash change) is not held, and a reload or a
 * closed tab gets the browser's own `beforeunload` prompt. A held navigation
 * whose rows were saved or closed meanwhile goes on by itself, as Render's
 * does. Mounted once, inside the router, by the root component.
 */
export function UnsavedChangesProvider({ children }: { children: ReactNode }) {
  const { t, i18n } = useTranslations();
  const [fields, setFields] = useState<ReadonlyMap<string, UnsavedField>>(
    () => new Map(),
  );
  const held = fields.size > 0;

  const shouldBlockFn = useCallback<ShouldBlockFn>(
    ({ current, next }) => held && current.pathname !== next.pathname,
    [held],
  );
  const blocker = useBlocker({
    shouldBlockFn,
    enableBeforeUnload: held,
    withResolver: true,
  });

  // Keep editing hands focus back to the first edited field once the dialog
  // has closed (ConfirmDialog's return focus); Discard hands it to nobody.
  const focusTarget = useRef<HTMLElement | null>(null);
  // Discard closes the dialog too; that close is not a Keep editing.
  const discarding = useRef(false);

  useEffect(() => {
    if (blocker.status !== "blocked" || held) return;
    focusTarget.current = null;
    blocker.proceed();
  }, [blocker, held]);

  const register = useCallback((id: string, field: UnsavedField) => {
    setFields((current) => new Map(current).set(id, field));
  }, []);
  const unregister = useCallback((id: string) => {
    setFields((current) => {
      if (!current.has(id)) return current;
      const next = new Map(current);
      next.delete(id);
      return next;
    });
  }, []);
  const registry = useMemo<UnsavedChangesRegistry>(
    () => ({ register, unregister }),
    [register, unregister],
  );

  const edited = [...fields.values()];
  const open = blocker.status === "blocked" && held;
  const [before, after] = aroundPlaceholder(
    t("common.unsavedChangesBody", { fields: PLACEHOLDER_SENTINEL }),
  );
  const named = open
    ? editedFieldsPhrase(
        t,
        i18n.language,
        edited.map((field) => field.label),
      )
    : "";

  function keepEditing() {
    focusTarget.current = edited[0]?.target() ?? null;
    blocker.reset?.();
  }

  return (
    <UnsavedChangesContext.Provider value={registry}>
      {children}
      <ConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (next) return;
          if (discarding.current) discarding.current = false;
          else keepEditing();
        }}
        title={t("common.unsavedChangesTitle")}
        description={
          <>
            {before}
            <strong className="text-foreground font-semibold">{named}</strong>
            {after}
          </>
        }
        cancelLabel={t("common.unsavedChangesKeepEditing")}
        confirmLabel={t("common.unsavedChangesDiscard")}
        returnFocusRef={focusTarget}
        onConfirm={() => {
          discarding.current = true;
          focusTarget.current = null;
          blocker.proceed?.();
        }}
      />
    </UnsavedChangesContext.Provider>
  );
}
