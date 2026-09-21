import { useCallback, useRef, useState } from "react";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { protectedServiceName } from "@/features/services/lib/protected-confirmation";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  type AskForConfirmation,
  ProtectedRetryContext,
} from "@/common/providers/protected-retry-context";

interface PendingAsk {
  phrase: string;
  resolve: (confirmation: string | null) => void;
}

/**
 * The one mount of the protected-environment retry dialog (w4/m126). See
 * protected-retry-context.ts for why it lives above the routes instead of in
 * each Settings editor.
 */
export function ProtectedRetryProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const { t } = useTranslations();
  const [pending, setPending] = useState<PendingAsk | null>(null);
  // The resolver has to survive the state update that closes the dialog, or a
  // dismissal would leave the caller's promise hanging forever.
  const pendingRef = useRef<PendingAsk | null>(null);

  const settle = useCallback((confirmation: string | null) => {
    pendingRef.current?.resolve(confirmation);
    pendingRef.current = null;
    setPending(null);
  }, []);

  const ask = useCallback<AskForConfirmation>((phrase) => {
    // One dialog at a time: a second refusal while one is open means an earlier
    // save is still unresolved, and stacking dialogs would hide it.
    pendingRef.current?.resolve(null);
    return new Promise<string | null>((resolve) => {
      const next = { phrase, resolve };
      pendingRef.current = next;
      setPending(next);
    });
  }, []);

  return (
    <ProtectedRetryContext.Provider value={ask}>
      {children}
      <ProtectedConfirmationDialog
        open={pending !== null}
        resourceName={pending ? protectedServiceName(pending.phrase) : ""}
        requiredConfirmation={pending?.phrase ?? ""}
        actionLabel={t("common.protectedConfirmationRetry")}
        busy={false}
        onOpenChange={(next) => {
          if (!next) settle(null);
        }}
        onConfirm={async (confirmation) => {
          settle(confirmation);
        }}
      />
    </ProtectedRetryContext.Provider>
  );
}
