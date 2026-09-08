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
import { useActionAccess } from "./use-action-access";
import {
  SafeActionFeedbackView,
  type SafeActionFeedbackMessages,
} from "./safe-action-feedback";

export type MobileActionRunResult =
  | { status: "success" }
  | { status: "accepted_unverified" }
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
  const access = useActionAccess();
  const binding = useRef<string | null>(null);
  const optionsRef = useRef(options);
  optionsRef.current = options;
  const sequence = useRef(0);
  const viewSequence = sequence.current;
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

  function currentOption() {
    if (
      !selected ||
      !binding.current ||
      !access.isCurrent(binding.current, selected.definition.id)
    )
      return undefined;
    return optionsRef.current.find(
      (option) =>
        option.key === selected.key &&
        option.definition.id === selected.definition.id &&
        option.target.kind === selected.target.kind &&
        option.target.id === selected.target.id,
    );
  }

  useEffect(() => {
    if (!selected || currentOption()) return;
    executor.cancelAll();
    sequence.current += 1;
    binding.current = null;
    setIntent(null);
    setSelected(null);
    setServerConfirmation(null);
    setOutcome(null);
    setPending(false);
  });

  function request(option: MobileActionOption) {
    if (!access.isCurrent(access.key, option.definition.id)) return;
    binding.current = access.key;
    sequence.current += 1;
    setSelected({ ...option, target: Object.freeze({ ...option.target }) });
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
    if (!selected || !intent || pending || viewSequence !== sequence.current)
      return;
    const option = currentOption();
    if (!option) return;
    const confirmedBinding = binding.current;
    const confirmed = intent.confirmed ? intent : confirmSafeAction(intent);
    setIntent(confirmed);
    setPending(true);
    try {
      const next = await executor.execute<MobileActionRunResult>(
        confirmed,
        async () => {
          if (
            !confirmedBinding ||
            !access.isCurrent(confirmedBinding, confirmed.actionId)
          ) {
            return { data: { status: "not_allowed" } };
          }
          const result = await option.run(
            serverConfirmation ?? undefined,
            confirmed.retryIdentity,
          );
          switch (result.status) {
            case "success":
            case "confirmation_required":
              return { data: result };
            case "accepted_unverified":
              return { data: result, feedback: "accepted-unverified" };
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
      if (
        viewSequence !== sequence.current ||
        !confirmedBinding ||
        !access.isCurrent(confirmedBinding, confirmed.actionId)
      )
        return;
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
      if (viewSequence === sequence.current) setPending(false);
    }
  }

  const messages: SafeActionFeedbackMessages = {
    success: t("safeActions.feedback.success"),
    "accepted-unverified": t("safeActions.feedback.acceptedUnverified"),
    "authorization-denied": t("safeActions.feedback.authorizationDenied"),
    conflict: t("safeActions.feedback.conflict"),
    "timeout-unknown": t("safeActions.feedback.timeoutUnknown"),
    "audit-pending": t("safeActions.feedback.auditPending"),
    "audit-unavailable": t("safeActions.feedback.auditUnavailable"),
    failed: t("safeActions.feedback.failed"),
    canceled: t("safeActions.feedback.canceled"),
    ...feedbackMessages,
  };

  const blockedOption = options.find(
    (option) => !access.isCurrent(access.key, option.definition.id),
  );
  return (
    <View style={styles.container}>
      {blockedOption ? (
        <Text
          accessibilityRole="alert"
          style={{ color: theme.mutedForeground }}
        >
          {t(access.reason(blockedOption.definition.id))}
        </Text>
      ) : null}
      <View style={styles.buttons}>
        {options.length ? (
          options.map((option) => (
            <Button
              key={option.key}
              type="outline"
              style={styles.button}
              disabled={
                pending || !access.isCurrent(access.key, option.definition.id)
              }
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
          sequence.current += 1;
          binding.current = null;
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
