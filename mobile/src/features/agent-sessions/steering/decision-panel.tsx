import { useEffect, useMemo, useState } from "react";
import {
  Pressable,
  StyleSheet,
  Switch,
  Text,
  TextInput,
  View,
} from "react-native";
import * as WebBrowser from "expo-web-browser";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import type { MobileAgentSessionDecisionsQuery } from "@/generated-graphql";
import {
  buildDecisionResponse,
  initialDecisionValues,
  parseDecisionContract,
  type DecisionField,
  type DecisionOption,
} from "./decision-contract";

export type MobileDecision =
  MobileAgentSessionDecisionsQuery["agentSessionDecisions"][number];

export function DecisionPanel({
  decision,
  submitting,
  error,
  onRespond,
}: {
  decision: MobileDecision;
  submitting: boolean;
  error: boolean;
  onRespond: (
    action: "accept" | "deny" | "cancel",
    value?: Record<string, unknown>,
  ) => Promise<void>;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const contract = useMemo(
    () => parseDecisionContract(decision.responseSchemaJson),
    [decision.responseSchemaJson],
  );
  const fields = contract.ok ? contract.fields : [];
  const [values, setValues] = useState<Record<string, unknown>>(() =>
    initialDecisionValues(fields),
  );
  const [invalid, setInvalid] = useState(false);
  useEffect(() => {
    setValues(initialDecisionValues(fields));
    setInvalid(false);
  }, [decision.id]); // fields belong to this immutable decision version.

  const submitForm = async () => {
    const built = buildDecisionResponse(fields, values);
    if (!built.ok) {
      setInvalid(true);
      return;
    }
    setInvalid(false);
    await onRespond("accept", built.value);
  };

  return (
    <DashboardCard title={t("agentSessions.decision.title")}>
      <View testID={`agent-decision-${decision.id}`} style={styles.stack}>
        <Text style={[styles.message, { color: theme.foreground }]}>
          {decision.message}
        </Text>
        <Text style={[styles.meta, { color: theme.mutedForeground }]}>
          {t("agentSessions.decision.turnExpires", {
            time: new Date(decision.expiresAt).toLocaleTimeString([], {
              hour: "numeric",
              minute: "2-digit",
            }),
          })}
        </Text>

        {decision.type === "permission" ? (
          <PermissionActions
            actions={decision.actions}
            disabled={submitting}
            onSelect={(optionId) => void onRespond("accept", { optionId })}
          />
        ) : decision.type === "elicitation_url" ? (
          <UrlDecision
            url={decision.externalUrl}
            disabled={submitting}
            onComplete={() => void onRespond("accept", {})}
            onDecline={() => void onRespond("deny")}
          />
        ) : decision.type === "clarification" ||
          decision.type === "elicitation_form" ? (
          contract.ok ? (
            <View style={styles.stack}>
              {fields.map((field) => (
                <DecisionFieldControl
                  key={field.name}
                  field={field}
                  value={values[field.name]}
                  disabled={submitting}
                  onChange={(value) =>
                    setValues((current) => ({
                      ...current,
                      [field.name]: value,
                    }))
                  }
                />
              ))}
              <View style={styles.actions}>
                <Button
                  type="outline"
                  style={styles.action}
                  disabled={submitting}
                  onPress={() => void onRespond("deny")}
                >
                  {t("agentSessions.decision.decline")}
                </Button>
                <Button
                  style={styles.action}
                  loading={submitting}
                  disabled={submitting}
                  onPress={() => void submitForm()}
                >
                  {t("agentSessions.decision.submit")}
                </Button>
              </View>
            </View>
          ) : (
            <Text accessibilityRole="alert" style={{ color: theme.warning }}>
              {contract.reason === "sensitive"
                ? t("agentSessions.decision.sensitiveUnsupported")
                : t("agentSessions.decision.unsupported")}
            </Text>
          )
        ) : (
          <Text accessibilityRole="alert" style={{ color: theme.warning }}>
            {t("agentSessions.decision.unsupported")}
          </Text>
        )}

        {invalid ? (
          <Text accessibilityRole="alert" style={{ color: theme.warning }}>
            {t("agentSessions.decision.invalid")}
          </Text>
        ) : null}
        {error ? (
          <Text accessibilityRole="alert" style={{ color: theme.warning }}>
            {t("agentSessions.decision.changed")}
          </Text>
        ) : null}
      </View>
    </DashboardCard>
  );
}

function PermissionActions({
  actions,
  disabled,
  onSelect,
}: {
  actions: MobileDecision["actions"];
  disabled: boolean;
  onSelect: (id: string) => void;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const safeActions = actions.filter(
    (action) => action.kind === "allow_once" || action.kind === "reject_once",
  );
  if (safeActions.length === 0) {
    return (
      <Text accessibilityRole="alert" style={{ color: theme.warning }}>
        {t("agentSessions.decision.unsupported")}
      </Text>
    );
  }
  return (
    <View style={styles.stack}>
      {safeActions.map((action) => (
        <Button
          key={action.id}
          type={action.kind === "allow_once" ? "primary" : "outline"}
          disabled={disabled}
          onPress={() => onSelect(action.id)}
          accessibilityLabel={action.label}
        >
          {action.label}
        </Button>
      ))}
      <Text style={[styles.meta, { color: theme.mutedForeground }]}>
        {t("agentSessions.decision.onceOnly")}
      </Text>
    </View>
  );
}

function UrlDecision({
  url,
  disabled,
  onComplete,
  onDecline,
}: {
  url?: string | null;
  disabled: boolean;
  onComplete: () => void;
  onDecline: () => void;
}) {
  const { t } = useTranslations();
  const safeUrl = safeExternalUrl(url);
  if (!safeUrl) {
    return (
      <Text accessibilityRole="alert">
        {t("agentSessions.decision.unsupported")}
      </Text>
    );
  }
  return (
    <View style={styles.stack}>
      <Button
        type="outline"
        disabled={disabled}
        onPress={() => void WebBrowser.openBrowserAsync(safeUrl)}
      >
        {t("agentSessions.decision.openLink")}
      </Button>
      <View style={styles.actions}>
        <Button
          type="outline"
          style={styles.action}
          disabled={disabled}
          onPress={onDecline}
        >
          {t("agentSessions.decision.decline")}
        </Button>
        <Button style={styles.action} disabled={disabled} onPress={onComplete}>
          {t("agentSessions.decision.complete")}
        </Button>
      </View>
    </View>
  );
}

function DecisionFieldControl({
  field,
  value,
  disabled,
  onChange,
}: {
  field: DecisionField;
  value: unknown;
  disabled: boolean;
  onChange: (value: unknown) => void;
}) {
  const theme = useTheme().colorTheme;
  return (
    <View style={styles.field}>
      <Text style={[styles.label, { color: theme.foreground }]}>
        {field.title}
        {field.required ? " *" : ""}
      </Text>
      {field.description ? (
        <Text style={[styles.meta, { color: theme.mutedForeground }]}>
          {field.description}
        </Text>
      ) : null}
      {field.type === "boolean" ? (
        <Switch
          value={value === true}
          disabled={disabled}
          onValueChange={onChange}
          trackColor={{ true: theme.primary }}
        />
      ) : field.options.length > 0 ? (
        <View style={styles.options}>
          {field.options.map((option) => (
            <Option
              key={option.value}
              option={option}
              selected={
                field.type === "array"
                  ? Array.isArray(value) && value.includes(option.value)
                  : value === option.value
              }
              disabled={disabled}
              onPress={() => {
                if (field.type !== "array") {
                  onChange(option.value);
                  return;
                }
                const current = Array.isArray(value)
                  ? value.filter(
                      (item): item is string => typeof item === "string",
                    )
                  : [];
                onChange(
                  current.includes(option.value)
                    ? current.filter((item) => item !== option.value)
                    : [...current, option.value],
                );
              }}
            />
          ))}
        </View>
      ) : (
        <TextInput
          value={
            typeof value === "string" || typeof value === "number"
              ? String(value)
              : ""
          }
          editable={!disabled}
          onChangeText={onChange}
          keyboardType={
            field.type === "integer"
              ? "number-pad"
              : field.type === "number"
                ? "decimal-pad"
                : "default"
          }
          multiline={field.type === "string"}
          style={[
            styles.input,
            field.type === "string" && styles.multiline,
            { color: theme.foreground, borderColor: theme.border },
          ]}
        />
      )}
    </View>
  );
}

function Option({
  option,
  selected,
  disabled,
  onPress,
}: {
  option: DecisionOption;
  selected: boolean;
  disabled: boolean;
  onPress: () => void;
}) {
  const theme = useTheme().colorTheme;
  return (
    <Pressable
      accessibilityRole="radio"
      accessibilityState={{ selected, disabled }}
      disabled={disabled}
      onPress={onPress}
      style={[
        styles.option,
        { borderColor: selected ? theme.primary : theme.border },
        selected && { backgroundColor: theme.primaryMuted },
      ]}
    >
      <Text style={{ color: selected ? theme.primary : theme.foreground }}>
        {option.label}
      </Text>
    </Pressable>
  );
}

export function safeExternalUrl(
  value: string | null | undefined,
): string | null {
  if (!value) return null;
  try {
    const parsed = new URL(value);
    return parsed.protocol === "https:" &&
      parsed.host &&
      !parsed.username &&
      !parsed.password
      ? parsed.toString()
      : null;
  } catch {
    return null;
  }
}

const styles = StyleSheet.create({
  stack: { gap: space.md },
  message: {
    fontSize: fontSizes.md,
    lineHeight: fontSizes.md * 1.5,
    fontWeight: fontWeights.medium,
  },
  meta: { fontSize: fontSizes.xs, lineHeight: fontSizes.xs * 1.5 },
  actions: { flexDirection: "row", gap: space.sm },
  action: { flex: 1 },
  field: { gap: space.xs },
  label: { fontSize: fontSizes.sm, fontWeight: fontWeights.medium },
  input: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: space.sm,
    paddingHorizontal: space.sm,
    paddingVertical: space.sm,
    fontSize: fontSizes.md,
  },
  multiline: { minHeight: 72, textAlignVertical: "top" },
  options: { gap: space.xs },
  option: {
    minHeight: 44,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: space.sm,
    justifyContent: "center",
    paddingHorizontal: space.md,
  },
});
