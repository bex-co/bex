import { ScreenToolbar } from "@/components/screen-toolbar";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { router } from "expo-router";
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Linking,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { SafeAreaProvider, SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { Picker } from "@/components/picker";
import { SearchableListModal } from "@/components/searchable-list";
import { TextInputModal } from "@/components/text-input-modal";
import {
  SafeActionConfirmationDialog,
  SafeActionFeedbackView,
  defineSafeAction,
  useSafeAction,
  type SafeActionFeedbackMessages,
} from "@/components/safe-action";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import type { ColorTheme } from "@/types/theme-props";
import {
  MobileAgentReposDocument,
  MobileAgentSessionCapabilitiesDocument,
  MobileCreateAgentSessionDocument,
} from "@/generated-graphql";
import {
  buildCreateVariables,
  canSubmit,
  defaultBranchFor,
  type ComposerFields,
} from "./compose";
import {
  loadRepoFrequency,
  rankRepos,
  recordRepoUse,
  type RepoFrequency,
} from "./repo-frequency";
import { isGitHubUrl } from "../detail/github-links";

const createAgentSession = defineSafeAction(
  "create-agent-session",
  "agent-session",
);

// Only open an https://github.com/ target, never an arbitrary URL a readiness
// payload might smuggle in (shared guard with the detail draft-PR link).
function openGitHub(url: string | null | undefined): void {
  if (isGitHubUrl(url)) void Linking.openURL(url!);
}

// The bare repo name (last path segment) for the compact pill; the full
// owner/name still submits and drives selection.
function shortRepo(full: string): string {
  const parts = full.split("/");
  return parts[parts.length - 1] || full;
}

type OpenPicker = "repo" | "branch" | "agent" | null;

export function SessionComposer({ onClose }: { onClose: () => void }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { selected } = useWorkspace();
  const ownerId = selected?.id ?? "";

  const capabilities = useQuery(MobileAgentSessionCapabilitiesDocument, {
    variables: { ownerId },
    skip: !selected,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const repos = useQuery(MobileAgentReposDocument, {
    variables: { ownerId },
    skip: !selected,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const [createSession] = useMutation(MobileCreateAgentSessionDocument);

  const caps = capabilities.data?.agentSessionCapabilities;
  const agents = caps?.agents ?? [];
  const ready = caps?.ready ?? false;
  const enabled = caps?.enabled ?? false;

  const [fields, setFields] = useState<ComposerFields>({
    repo: "",
    branch: "",
    prompt: "",
    agent: "",
  });
  const [agentTouched, setAgentTouched] = useState(false);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const [openPicker, setOpenPicker] = useState<OpenPicker>(null);
  const [repoFreq, setRepoFreq] = useState<RepoFrequency>({});
  const set = (patch: Partial<ComposerFields>) =>
    setFields((prev) => ({ ...prev, ...patch }));

  const repoList = useMemo(
    () => (repos.data?.repos ?? []).filter((r) => r?.fullName),
    [repos.data],
  );

  // Select a repo by name and adopt its default branch (shared by the quick
  // pills and the full picker).
  const selectRepo = (fullName: string) => {
    const repo = repoList.find((r) => r!.fullName === fullName);
    set({ repo: fullName, branch: defaultBranchFor(repo?.defaultBranch) });
  };

  // Load this workspace's recorded repo usage for the quick-select pills.
  useEffect(() => {
    if (!ownerId) return;
    let active = true;
    void loadRepoFrequency(ownerId).then((freq) => {
      if (active) setRepoFreq(freq);
    });
    return () => {
      active = false;
    };
  }, [ownerId]);

  // Up to four most-used repos (recency breaks ties), filled from the list so
  // the row is useful before any history exists.
  const quickRepos = useMemo(
    () =>
      rankRepos(
        repoFreq,
        repoList.map((r) => r!.fullName!),
        4,
      ),
    [repoFreq, repoList],
  );

  // Prefill the repo (with its default branch) and agent so the composer is a
  // single "ask + send" flow; both remain one tap away from a different pick.
  useEffect(() => {
    if (!ready) return;
    setFields((prev) => {
      let next = prev;
      if (!prev.repo && repoList.length) {
        const first = repoList[0]!;
        next = {
          ...next,
          repo: first.fullName!,
          branch: defaultBranchFor(first.defaultBranch),
        };
      }
      if (!prev.agent && agents.length) {
        next = { ...next, agent: agents[0].id };
      }
      return next;
    });
  }, [ready, repoList, agents]);

  const submit = useSafeAction<{ id: string }>({
    operation: async () => {
      const result = await createSession({
        variables: buildCreateVariables(ownerId, fields),
      });
      const id = result.data?.createAgentSession?.id ?? null;
      if (!id) throw new Error("no session id");
      setCreatedId(id);
      void recordRepoUse(ownerId, fields.repo, Date.now()).then(setRepoFreq);
      return { data: { id }, feedback: "accepted-unverified" };
    },
  });

  const canGo =
    canSubmit({ fields, ready, submitting: submit.pending }) &&
    createdId === null;

  const onSubmit = () => {
    if (!canGo) return;
    submit.requestConfirmation(createAgentSession, {
      kind: "agent-session",
      id: fields.repo,
      label: fields.repo,
    });
  };

  const feedbackMessages: SafeActionFeedbackMessages = {
    success: t("safeActions.feedback.success"),
    "accepted-unverified": t("safeActions.feedback.acceptedUnverified"),
    "authorization-denied": t("safeActions.feedback.authorizationDenied"),
    conflict: t("safeActions.feedback.conflict"),
    "timeout-unknown": t("safeActions.feedback.timeoutUnknown"),
    "audit-pending": t("safeActions.feedback.auditPending"),
    "audit-unavailable": t("safeActions.feedback.auditUnavailable"),
    failed: t("safeActions.feedback.failed"),
    canceled: t("safeActions.feedback.canceled"),
  };

  const loadingCaps = capabilities.loading && !capabilities.data;
  const composing = enabled && ready && !createdId;

  const agentLabel = !agentTouched
    ? t("agentSessions.composer.agentAuto")
    : (agents.find((a) => a.id === fields.agent)?.label ??
      t("agentSessions.composer.agentAuto"));

  return (
    <Modal animationType="slide" onRequestClose={onClose}>
      <SafeAreaProvider>
        <SafeAreaView
          style={[styles.safe, { backgroundColor: theme.background }]}
        >
          <ScreenToolbar
            title={t("agentSessions.composer.title")}
            left={
              <Pressable
                accessibilityRole="button"
                accessibilityLabel={t("agentSessions.composer.close")}
                onPress={onClose}
                hitSlop={8}
                style={styles.headerSide}
              >
                <Ionicons name="close" size={24} color={theme.foreground} />
              </Pressable>
            }
            right={
              composing ? (
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel={t("agentSessions.composer.submit")}
                  accessibilityState={{ disabled: !canGo }}
                  disabled={!canGo}
                  onPress={onSubmit}
                  hitSlop={8}
                  style={[
                    styles.sendButton,
                    {
                      backgroundColor: canGo ? theme.primary : theme.border,
                    },
                  ]}
                >
                  <Ionicons
                    name="arrow-up"
                    size={22}
                    color={canGo ? theme.onPrimary : theme.mutedForeground}
                  />
                </Pressable>
              ) : null
            }
          />

          {composing ? (
            <KeyboardAvoidingView
              style={styles.flex}
              behavior={Platform.OS === "ios" ? "padding" : undefined}
            >
              <TextInput
                value={fields.prompt}
                onChangeText={(prompt) => set({ prompt })}
                placeholder={t("agentSessions.composer.promptPlaceholder")}
                placeholderTextColor={theme.mutedForeground}
                multiline
                autoFocus
                style={[styles.prompt, { color: theme.foreground }]}
                textAlignVertical="top"
              />
              {quickRepos.length > 1 ? (
                <ScrollView
                  horizontal
                  showsHorizontalScrollIndicator={false}
                  keyboardShouldPersistTaps="handled"
                  style={styles.quickRow}
                  contentContainerStyle={styles.quickRowContent}
                >
                  {quickRepos.map((name) => {
                    const active = fields.repo === name;
                    return (
                      <Pressable
                        key={name}
                        accessibilityRole="button"
                        accessibilityState={{ selected: active }}
                        accessibilityLabel={name}
                        onPress={() => selectRepo(name)}
                        style={[
                          styles.quickChip,
                          {
                            borderColor: active ? theme.primary : theme.border,
                            backgroundColor: active
                              ? theme.primary
                              : "transparent",
                          },
                        ]}
                      >
                        <Text
                          numberOfLines={1}
                          style={[
                            styles.quickChipText,
                            { color: active ? theme.white : theme.foreground },
                          ]}
                        >
                          {shortRepo(name)}
                        </Text>
                      </Pressable>
                    );
                  })}
                </ScrollView>
              ) : null}
              <View style={styles.pillRow}>
                <Pill
                  theme={theme}
                  icon={
                    <View
                      style={[
                        styles.repoIcon,
                        { backgroundColor: theme.primary },
                      ]}
                    >
                      <Ionicons name="cube" size={12} color={theme.white} />
                    </View>
                  }
                  label={
                    fields.repo
                      ? shortRepo(fields.repo)
                      : t("agentSessions.composer.repo")
                  }
                  accessibilityLabel={t("agentSessions.composer.repo")}
                  onPress={() => setOpenPicker("repo")}
                  shrink
                />
                <Pill
                  theme={theme}
                  icon={
                    <Ionicons
                      name="git-branch"
                      size={15}
                      color={theme.foreground}
                    />
                  }
                  label={fields.branch || "main"}
                  accessibilityLabel={t("agentSessions.composer.branch")}
                  onPress={() => setOpenPicker("branch")}
                />
                <Pill
                  theme={theme}
                  label={agentLabel}
                  accessibilityLabel={t("agentSessions.composer.agent")}
                  onPress={() => setOpenPicker("agent")}
                />
              </View>
            </KeyboardAvoidingView>
          ) : (
            <ScrollView contentContainerStyle={styles.content}>
              {loadingCaps ? (
                <DashboardCard>
                  <ActivityIndicator color={theme.primary} />
                </DashboardCard>
              ) : createdId ? (
                <DashboardCard>
                  <Text style={[styles.title, { color: theme.foreground }]}>
                    {t("agentSessions.composer.submitted")}
                  </Text>
                  <Button
                    onPress={() => {
                      onClose();
                      router.push(`/sessions/${createdId}`);
                    }}
                    accessibilityLabel={t("agentSessions.composer.openSession")}
                  >
                    {t("agentSessions.composer.openSession")}
                  </Button>
                </DashboardCard>
              ) : (
                <DashboardCard>
                  <Text style={[styles.title, { color: theme.foreground }]}>
                    {t("agentSessions.composer.setupTitle")}
                  </Text>
                  <Text style={[styles.body, { color: theme.mutedForeground }]}>
                    {!caps?.github.connected
                      ? t("agentSessions.composer.needGithub")
                      : !caps?.modelKeyReady
                        ? t("agentSessions.composer.needModelKey")
                        : t("agentSessions.composer.needDesktop")}
                  </Text>
                  {!caps?.github.connected && caps?.github.installUrl ? (
                    <Button
                      type="outline"
                      onPress={() => openGitHub(caps?.github.installUrl)}
                      accessibilityLabel={t(
                        "agentSessions.composer.connectGithub",
                      )}
                    >
                      {t("agentSessions.composer.connectGithub")}
                    </Button>
                  ) : null}
                </DashboardCard>
              )}
            </ScrollView>
          )}
        </SafeAreaView>
      </SafeAreaProvider>

      <SearchableListModal
        visible={openPicker === "repo"}
        title={t("agentSessions.composer.repo")}
        searchPlaceholder={t("agentSessions.composer.searchRepo")}
        cancelLabel={t("agentSessions.composer.close")}
        emptyLabel={t("agentSessions.composer.searchEmpty")}
        items={repoList.map((r) => ({
          label: r!.fullName!,
          value: r!.fullName!,
        }))}
        selectedValue={fields.repo}
        onSelect={(value) => {
          selectRepo(value);
          setOpenPicker(null);
        }}
        onCancel={() => setOpenPicker(null)}
      />
      <Picker
        visible={openPicker === "agent"}
        title={t("agentSessions.composer.agent")}
        items={agents.map((a) => ({ label: a.label, value: a.id }))}
        selectedValue={fields.agent}
        onSelect={(item) => {
          set({ agent: item.value });
          setAgentTouched(true);
          setOpenPicker(null);
        }}
        onCancel={() => setOpenPicker(null)}
      />
      <TextInputModal
        visible={openPicker === "branch"}
        title={t("agentSessions.composer.branch")}
        placeholder={fields.branch || "main"}
        onConfirm={(text) => {
          const branch = text.trim();
          if (branch) set({ branch });
          setOpenPicker(null);
        }}
        onCancel={() => setOpenPicker(null)}
      />

      <SafeActionConfirmationDialog
        intent={submit.intent}
        pending={submit.pending}
        title={t("safeActions.title")}
        message={t("safeActions.message")}
        actionLabel={t("agentSessions.composer.submit")}
        confirmLabel={t("safeActions.confirm")}
        cancelLabel={t("safeActions.cancel")}
        pendingLabel={t("safeActions.pending")}
        onConfirm={() => void submit.confirm()}
        onCancel={submit.dismissConfirmation}
      />
      <SafeActionFeedbackView
        outcome={submit.outcome}
        messages={feedbackMessages}
        retryLabel={t("safeActions.refreshFirst")}
        dismissLabel={t("safeActions.dismiss")}
        onRetry={() => undefined}
        onDismiss={submit.dismissOutcome}
      />
    </Modal>
  );
}

function Pill({
  theme,
  icon,
  label,
  accessibilityLabel,
  onPress,
  shrink,
}: {
  theme: ColorTheme;
  icon?: ReactNode;
  label: string;
  accessibilityLabel: string;
  onPress: () => void;
  shrink?: boolean;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      onPress={onPress}
      style={[
        styles.pill,
        shrink && styles.pillShrink,
        { borderColor: theme.border },
      ]}
    >
      {icon}
      <Text
        numberOfLines={1}
        style={[styles.pillText, { color: theme.foreground }]}
      >
        {label}
      </Text>
      <Ionicons name="chevron-down" size={14} color={theme.mutedForeground} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  flex: { flex: 1 },
  headerSide: {
    width: 44,
    minHeight: 44,
    justifyContent: "center",
    alignItems: "center",
  },
  sendButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: "center",
    justifyContent: "center",
  },
  prompt: {
    flex: 1,
    padding: gutter,
    fontSize: fontSizes.lg,
  },
  quickRow: { flexGrow: 0, height: 48 },
  quickRowContent: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
    paddingHorizontal: gutter,
  },
  quickChip: {
    height: 32,
    justifyContent: "center",
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 16,
    paddingHorizontal: space.md,
  },
  quickChipText: { fontSize: fontSizes.sm, maxWidth: 160 },
  pillRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
    paddingHorizontal: gutter,
    paddingVertical: space.sm,
  },
  pill: {
    flexDirection: "row",
    alignItems: "center",
    alignSelf: "center",
    height: 40,
    flexShrink: 0,
    gap: space.xs,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 20,
    paddingHorizontal: space.md,
  },
  pillShrink: { flexShrink: 1 },
  pillText: { fontSize: fontSizes.sm, flexShrink: 1 },
  repoIcon: {
    width: 20,
    height: 20,
    borderRadius: 5,
    alignItems: "center",
    justifyContent: "center",
  },
  content: { padding: gutter, gap: space.md },
  title: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  body: {
    fontSize: fontSizes.sm,
    lineHeight: fontSizes.sm * 1.5,
    marginVertical: space.xs,
  },
});
