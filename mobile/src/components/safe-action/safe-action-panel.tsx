import { Fragment, useEffect, useRef, useState, type ReactNode } from "react";
import {
  StyleSheet,
  Text,
  View,
  type StyleProp,
  type ViewStyle,
} from "react-native";
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

export type SafeActionTriggerProps = {
  disabled: boolean;
  label: string;
  pending: boolean;
  onPress: () => void;
};

let retrySequence = 0;
function mobileRetryIdentity(): string {
  retrySequence += 1;
  return `mobile-${Date.now().toString(36)}-${retrySequence.toString(36)}`;
}

/** Shared single-flight, optional confirmation, and honest-result surface. */
export function SafeActionPanel({
  options,
  feedbackMessages,
  renderTrigger,
  emptyTriggerLabel,
  confirmationMode = "always",
  feedbackContainerStyle,
}: {
  options: MobileActionOption[];
  feedbackMessages?: Partial<SafeActionFeedbackMessages>;
  renderTrigger?: (props: SafeActionTriggerProps) => ReactNode;
  emptyTriggerLabel?: string;
  confirmationMode?: "always" | "server-only";
  feedbackContainerStyle?: StyleProp<ViewStyle>;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const executor = useRef(new SafeActionExecutor()).current;
  const pendingRef = useRef(false);
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
    if (pendingRef.current) return;
    const nextIntent = createSafeActionIntent(
      option.definition,
      option.target,
      mobileRetryIdentity,
    );
    setSelected(option);
    setServerConfirmation(null);
    setOutcome(null);
    if (confirmationMode === "server-only") {
      void execute(option, confirmSafeAction(nextIntent), null);
      return;
    }
    setIntent(nextIntent);
  }

  async function execute(
    option: MobileActionOption,
    confirmed: SafeActionIntent,
    confirmation: string | null,
  ) {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(true);
    try {
      const next = await executor.execute<MobileActionRunResult>(
        confirmed,
        async () => {
          const result = await option.run(
            confirmation ?? undefined,
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
      if (next.status === "succeeded" && isServerConfirmation(next.data)) {
        setServerConfirmation(next.data.confirmation);
        setOutcome(null);
        setIntent(
          createSafeActionIntent(
            option.definition,
            option.target,
            mobileRetryIdentity,
          ),
        );
        return;
      }
      setOutcome(next);
      setIntent(null);
    } finally {
      pendingRef.current = false;
      setPending(false);
    }
  }

  function confirm() {
    if (!selected || !intent || pendingRef.current) return;
    const confirmed = intent.confirmed ? intent : confirmSafeAction(intent);
    setIntent(confirmed);
    void execute(selected, confirmed, serverConfirmation);
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

  return (
    <View style={styles.container}>
      <View style={styles.buttons}>
        {options.length ? (
          options.map((option) => {
            const disabled = pending;
            const onPress = () => {
              if (!disabled) request(option);
            };
            return renderTrigger ? (
              <Fragment key={option.key}>
                {renderTrigger({
                  disabled,
                  label: option.label,
                  pending,
                  onPress,
                })}
              </Fragment>
            ) : (
              <Button
                key={option.key}
                type="outline"
                style={styles.button}
                disabled={disabled}
                accessibilityLabel={option.label}
                onPress={onPress}
              >
                {option.label}
              </Button>
            );
          })
        ) : renderTrigger && emptyTriggerLabel ? (
          renderTrigger({
            disabled: true,
            label: emptyTriggerLabel,
            pending: false,
            onPress: () => undefined,
          })
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
        onConfirm={confirm}
        onCancel={() => {
          setIntent(null);
          setSelected(null);
          setServerConfirmation(null);
        }}
      />
      {outcome ? (
        <View style={feedbackContainerStyle}>
          <SafeActionFeedbackView
            outcome={outcome}
            messages={messages}
            retryLabel={t("safeActions.refreshFirst")}
            dismissLabel={t("safeActions.dismiss")}
            onRetry={() => undefined}
            onDismiss={() => setOutcome(null)}
          />
        </View>
      ) : null}
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
