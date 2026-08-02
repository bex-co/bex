import { useEffect, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { Button } from "@/components/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { space, useTheme } from "@/common/theme";
import { SafeActionExecutor, type SafeActionOutcome } from "./executor";
import {
  confirmSafeAction,
  createSafeActionIntent,
  type SafeActionIntent,
  type SafeActionTarget,
} from "./model";
import type { SafeActionDefinition } from "./registry";
import { SafeActionConfirmationDialog } from "./safe-action-dialog";
import {
  SafeActionFeedbackView,
  type SafeActionFeedbackMessages,
} from "./safe-action-feedback";

export type MobileActionRunResult =
  | { status: "success" }
  | {
      status: "confirmation_required";
      source: "server";
      confirmation: string;
    }
  | { status: "busy" | "not_allowed" }
  | { status: "timeout" }
  | { status: "error"; error: unknown };

export type MobileActionOption = {
  key: string;
  definition: SafeActionDefinition;
  target: SafeActionTarget;
  label: string;
  run: (
    serverConfirmation?: string,
    retryIdentity?: string,
  ) => Promise<MobileActionRunResult>;
};

let retrySequence = 0;
function mobileRetryIdentity(): string {
  retrySequence += 1;
  return `mobile-${Date.now().toString(36)}-${retrySequence.toString(36)}`;
}

/** Shared confirmation, single-flight, and honest-result surface for m4. */
export function SafeActionPanel({
  options,
  feedbackMessages,
}: {
  options: MobileActionOption[];
  feedbackMessages?: Partial<SafeActionFeedbackMessages>;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const executor = useRef(new SafeActionExecutor()).current;
  const [selected, setSelected] = useState<MobileActionOption | null>(null);
  const [intent, setIntent] = useState<SafeActionIntent | null>(null);
  const [serverConfirmation, setServerConfirmation] = useState<string | null>(
    null,
  );
  const [pending, setPending] = useState(false);
  const [outcome, setOutcome] = useState<SafeActionOutcome<unknown> | null>(
    null,
  );

  useEffect(() => () => executor.cancelAll(), [executor]);

  function request(option: MobileActionOption) {
    setSelected(option);
    setServerConfirmation(null);
    setOutcome(null);
    setIntent(
      createSafeActionIntent(
        option.definition,
        option.target,
        mobileRetryIdentity,
      ),
    );
  }

  async function confirm() {
    if (!selected || !intent || pending) return;
    const confirmed = intent.confirmed ? intent : confirmSafeAction(intent);
    setIntent(confirmed);
    setPending(true);
    try {
      const next = await executor.execute<MobileActionRunResult>(
        confirmed,
        async () => {
          const result = await selected.run(
            serverConfirmation ?? undefined,
            confirmed.retryIdentity,
          );
          switch (result.status) {
            case "success":
            case "confirmation_required":
              return { data: result };
            case "timeout":
              throw Object.assign(new Error("action state remains unknown"), {
                name: "TimeoutError",
              });
            case "busy":
            case "not_allowed":
              throw Object.assign(new Error("resource state changed"), {
                statusCode: 409,
              });
            case "error":
              throw result.error;
          }
        },
      );
      if (next.status === "succeeded" && isServerConfirmation(next.data)) {
        setServerConfirmation(next.data.confirmation);
        setOutcome(null);
        setIntent(
          createSafeActionIntent(
            selected.definition,
            selected.target,
            mobileRetryIdentity,
          ),
        );
        return;
      }
      setOutcome(next);
      setIntent(null);
    } finally {
      setPending(false);
    }
  }

  const messages: SafeActionFeedbackMessages = {
    success: t("safeActions.feedback.success"),
    "authorization-denied": t("safeActions.feedback.authorizationDenied"),
    conflict: t("safeActions.feedback.conflict"),
    "timeout-unknown": t("safeActions.feedback.timeoutUnknown"),
    "audit-pending": t("safeActions.feedback.auditPending"),
    "audit-unavailable": t("safeActions.feedback.auditUnavailable"),
    failed: t("safeActions.feedback.failed"),
    canceled: t("safeActions.feedback.canceled"),
    ...feedbackMessages,
  };

  return (
    <View style={styles.container}>
      <View style={styles.buttons}>
        {options.length ? (
          options.map((option) => (
            <Button
              key={option.key}
              type="outline"
              style={styles.button}
              disabled={pending}
              accessibilityLabel={option.label}
              onPress={() => request(option)}
            >
              {option.label}
            </Button>
          ))
        ) : (
          <Text style={{ color: theme.mutedForeground }}>
            {t("safeActions.noneAvailable")}
          </Text>
        )}
      </View>
      <SafeActionConfirmationDialog
        intent={intent}
        pending={pending}
        title={t("safeActions.title")}
        message={
          serverConfirmation
            ? t("safeActions.serverConfirmation", {
                confirmation: serverConfirmation,
              })
            : t("safeActions.message")
        }
        actionLabel={selected?.label ?? ""}
        confirmLabel={t("safeActions.confirm")}
        cancelLabel={t("safeActions.cancel")}
        pendingLabel={t("safeActions.pending")}
        onConfirm={() => void confirm()}
        onCancel={() => {
          setIntent(null);
          setSelected(null);
          setServerConfirmation(null);
        }}
      />
      <SafeActionFeedbackView
        outcome={outcome}
        messages={messages}
        retryLabel={t("safeActions.refreshFirst")}
        dismissLabel={t("safeActions.dismiss")}
        onRetry={() => undefined}
        onDismiss={() => setOutcome(null)}
      />
    </View>
  );
}

function isServerConfirmation(
  result: MobileActionRunResult,
): result is Extract<
  MobileActionRunResult,
  { status: "confirmation_required" }
> {
  return result.status === "confirmation_required";
}

const styles = StyleSheet.create({
  container: { gap: space.md },
  buttons: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  button: { minWidth: 132, flexGrow: 1, paddingHorizontal: space.sm },
});
