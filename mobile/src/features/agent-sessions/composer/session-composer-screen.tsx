import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { router } from "expo-router";
import {
  ActivityIndicator,
  Keyboard,
  KeyboardAvoidingView,
  Linking,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  useWindowDimensions,
  View,
} from "react-native";
import { SafeAreaProvider, SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { MenuButton, type MenuButtonItem } from "@/components/menu-button";
import {
  SafeActionPanel,
  defineSafeAction,
  type MobileActionOption,
} from "@/components/safe-action";
import { TopBar } from "@/components/top-bar";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  gutter,
  maxFontSizeMultipliers,
  rowMinHeight,
  space,
  useTheme,
} from "@/common/theme";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import {
  MobileAgentReposDocument,
  MobileAgentSessionCapabilitiesDocument,
  MobileCreateAgentSessionDocument,
} from "@/generated-graphql";
import { isGitHubUrl } from "../detail/github-links";
import {
  buildCreateVariables,
  canSubmit,
  deriveBranch,
  repositoryDisplayName,
  type ComposerFields,
} from "./compose";
import { RepositoryPicker } from "./repository-picker";

const createAgentSession = defineSafeAction(
  "create-agent-session",
  "agent-session",
);

function openGitHub(url: string | null | undefined): void {
  if (isGitHubUrl(url)) void Linking.openURL(url!);
}

export function SessionComposer({ onClose }: { onClose: () => void }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { width } = useWindowDimensions();
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

  const [fields, setFields] = useState<Omit<ComposerFields, "agent">>({
    repo: "",
    branch: "",
    prompt: "",
  });
  const [agentSelection, setAgentSelection] = useState<
    { mode: "auto" } | { mode: "profile"; id: string }
  >({ mode: "auto" });
  const [repoPickerVisible, setRepoPickerVisible] = useState(false);
  const set = (patch: Partial<Omit<ComposerFields, "agent">>) =>
    setFields((previous) => ({ ...previous, ...patch }));

  const repoList = useMemo(
    () => (repos.data?.repos ?? []).filter((repo) => repo?.fullName),
    [repos.data],
  );
  const repoItems = useMemo(
    () =>
      repoList.map((repo) => ({
        label: repo!.fullName!,
        value: repo!.fullName!,
      })),
    [repoList],
  );

  const selectedAgent =
    agentSelection.mode === "profile"
      ? agents.find((agent) => agent.id === agentSelection.id)
      : undefined;
  const selectedAgentId = selectedAgent?.id || agents[0]?.id || "";
  const effectiveFields = { ...fields, agent: selectedAgentId };
  const submitEnabled = canSubmit({
    fields: effectiveFields,
    ready,
    submitting: false,
  });
  const selectedAgentLabel =
    selectedAgent?.label || t("agentSessions.composer.autoAgent");

  const options: MobileActionOption[] =
    enabled && ready && submitEnabled
      ? [
          {
            key: "agent:create",
            definition: createAgentSession,
            target: {
              kind: "agent-session",
              id: fields.repo,
              label: fields.repo,
            },
            label: t("agentSessions.composer.submit"),
            run: async () => {
              try {
                const result = await createSession({
                  variables: buildCreateVariables(ownerId, effectiveFields),
                });
                const id = result.data?.createAgentSession?.id ?? null;
                if (!id) {
                  return { status: "error", error: new Error("no session id") };
                }
                // Go straight into the session's chat window — no intermediate
                // "Session assigned / Open session" confirmation step.
                onClose();
                router.push(`/sessions/${id}`);
                return { status: "accepted_unverified" };
              } catch (error) {
                return { status: "error", error };
              }
            },
          },
        ]
      : [];

  const agentItems: MenuButtonItem[] = [
    {
      id: "auto",
      label: t("agentSessions.composer.autoAgent"),
      selected: agentSelection.mode === "auto",
      onPress: () => setAgentSelection({ mode: "auto" }),
      icon:
        agentSelection.mode === "auto" ? (
          <Ionicons name="checkmark" size={18} color={theme.primary} />
        ) : undefined,
    },
    ...agents.map((agent) => ({
      id: agent.id,
      label: agent.label,
      selected:
        agentSelection.mode === "profile" && agentSelection.id === agent.id,
      onPress: () => setAgentSelection({ mode: "profile", id: agent.id }),
      icon:
        agentSelection.mode === "profile" && agentSelection.id === agent.id ? (
          <Ionicons name="checkmark" size={18} color={theme.primary} />
        ) : undefined,
    })),
  ];

  const loadingCaps = capabilities.loading && !capabilities.data;
  const showEditor = enabled && ready && !loadingCaps;

  return (
    <Modal
      animationType="slide"
      presentationStyle="fullScreen"
      onRequestClose={onClose}
    >
      <SafeAreaProvider style={styles.safe}>
        <SafeAreaView
          testID="agent-session-composer"
          style={[styles.safe, { backgroundColor: theme.background }]}
        >
          {showEditor ? (
            <KeyboardAvoidingView
              behavior={Platform.OS === "ios" ? "padding" : undefined}
              style={styles.editor}
            >
              <View style={styles.editorHeader}>
                <View style={styles.headerSide}>
                  <Pressable
                    testID="agent-session-composer-close"
                    accessibilityRole="button"
                    accessibilityLabel={t("agentSessions.composer.close")}
                    onPress={onClose}
                    style={({ pressed }) => [
                      styles.roundAction,
                      {
                        backgroundColor: theme.card,
                        borderColor: theme.border,
                      },
                      pressed && styles.pressed,
                    ]}
                  >
                    <Ionicons name="close" size={24} color={theme.foreground} />
                  </Pressable>
                </View>
                <Text
                  numberOfLines={1}
                  maxFontSizeMultiplier={maxFontSizeMultipliers.heading}
                  style={[styles.editorTitle, { color: theme.foreground }]}
                >
                  {t("agentSessions.composer.title")}
                </Text>
                <View style={[styles.headerSide, styles.headerRight]}>
                  <SafeActionPanel
                    options={options}
                    emptyTriggerLabel={t("agentSessions.composer.submit")}
                    confirmationMode="server-only"
                    feedbackMessages={{
                      failed: t("agentSessions.composer.createFailed"),
                    }}
                    feedbackContainerStyle={[
                      styles.submitFeedback,
                      { width: Math.min(width - gutter * 2, 360) },
                    ]}
                    renderTrigger={({ disabled, label, pending, onPress }) => (
                      <Pressable
                        testID="agent-session-composer-submit"
                        accessibilityRole="button"
                        accessibilityLabel={label}
                        accessibilityState={{ disabled, busy: pending }}
                        disabled={disabled}
                        onPress={() => {
                          Keyboard.dismiss();
                          onPress();
                        }}
                        style={({ pressed }) => [
                          styles.roundAction,
                          {
                            backgroundColor: disabled
                              ? theme.black20
                              : theme.primary,
                            borderColor: disabled
                              ? theme.border
                              : theme.primary,
                          },
                          pressed && styles.pressed,
                        ]}
                      >
                        {pending ? (
                          <ActivityIndicator color={theme.white} />
                        ) : (
                          <Ionicons
                            name="arrow-up"
                            size={23}
                            color={
                              disabled ? theme.mutedForeground : theme.white
                            }
                          />
                        )}
                      </Pressable>
                    )}
                  />
                </View>
              </View>

              <TextInput
                testID="agent-session-prompt"
                value={fields.prompt}
                onChangeText={(prompt) => set({ prompt })}
                placeholder={t("agentSessions.composer.promptPlaceholder")}
                placeholderTextColor={theme.mutedForeground}
                multiline
                autoFocus
                blurOnSubmit={false}
                maxFontSizeMultiplier={maxFontSizeMultipliers.content}
                style={[styles.prompt, { color: theme.foreground }]}
              />

              <View style={styles.contextBar}>
                <Pressable
                  testID="agent-session-repository-select"
                  accessibilityRole="button"
                  accessibilityLabel={t("agentSessions.composer.chooseRepo")}
                  accessibilityState={{ expanded: repoPickerVisible }}
                  accessibilityValue={{ text: fields.repo || undefined }}
                  onPress={() => {
                    Keyboard.dismiss();
                    setRepoPickerVisible(true);
                  }}
                  style={({ pressed }) => [
                    styles.contextChip,
                    styles.repoChip,
                    {
                      backgroundColor: theme.card,
                      borderColor: theme.border,
                    },
                    pressed && styles.pressed,
                  ]}
                >
                  <Ionicons
                    name="logo-github"
                    size={18}
                    color={fields.repo ? theme.primary : theme.mutedForeground}
                  />
                  <Text
                    numberOfLines={1}
                    ellipsizeMode="middle"
                    maxFontSizeMultiplier={maxFontSizeMultipliers.control}
                    style={[
                      styles.chipLabel,
                      {
                        color: fields.repo
                          ? theme.foreground
                          : theme.mutedForeground,
                      },
                    ]}
                  >
                    {fields.repo
                      ? repositoryDisplayName(fields.repo)
                      : t("agentSessions.composer.repo")}
                  </Text>
                  <Ionicons
                    name="chevron-down"
                    size={16}
                    color={theme.mutedForeground}
                  />
                </Pressable>

                <View
                  style={[
                    styles.contextChip,
                    styles.branchChip,
                    { backgroundColor: theme.card, borderColor: theme.border },
                  ]}
                >
                  <Ionicons
                    name="git-branch-outline"
                    size={18}
                    color={theme.mutedForeground}
                  />
                  <TextInput
                    testID="agent-session-branch"
                    accessibilityLabel={t("agentSessions.composer.branch")}
                    value={fields.branch}
                    onChangeText={(branch) => set({ branch })}
                    placeholder={deriveBranch(fields.prompt)}
                    placeholderTextColor={theme.mutedForeground}
                    autoCapitalize="none"
                    autoCorrect={false}
                    selectTextOnFocus
                    numberOfLines={1}
                    maxFontSizeMultiplier={maxFontSizeMultipliers.control}
                    style={[styles.branchInput, { color: theme.foreground }]}
                  />
                </View>

                <MenuButton
                  testID="agent-session-agent-select"
                  accessibilityLabel={`${t("agentSessions.composer.agent")}: ${selectedAgentLabel}`}
                  items={agentItems}
                  placement="above"
                  icon={
                    <View
                      style={[
                        styles.contextChip,
                        styles.agentChip,
                        {
                          backgroundColor: theme.card,
                          borderColor: theme.border,
                        },
                      ]}
                    >
                      <Text
                        numberOfLines={1}
                        maxFontSizeMultiplier={maxFontSizeMultipliers.control}
                        style={[styles.chipLabel, { color: theme.foreground }]}
                      >
                        {selectedAgentLabel}
                      </Text>
                      <Ionicons
                        name="chevron-down"
                        size={16}
                        color={theme.mutedForeground}
                      />
                    </View>
                  }
                />
              </View>

              <RepositoryPicker
                visible={repoPickerVisible}
                items={repoItems}
                selectedValue={fields.repo}
                onCancel={() => setRepoPickerVisible(false)}
                onSelect={(item) => {
                  set({ repo: item.value });
                  setRepoPickerVisible(false);
                }}
              />
            </KeyboardAvoidingView>
          ) : (
            <>
              <TopBar
                title={t("agentSessions.composer.title")}
                showDrawer={false}
                showBell={false}
                right={
                  <Pressable
                    testID="agent-session-composer-close"
                    accessibilityRole="button"
                    accessibilityLabel={t("agentSessions.composer.close")}
                    onPress={onClose}
                    hitSlop={8}
                  >
                    <Ionicons name="close" size={22} color={theme.foreground} />
                  </Pressable>
                }
              />
              <ScrollView contentContainerStyle={styles.fallbackContent}>
                {loadingCaps ? (
                  <DashboardCard>
                    <ActivityIndicator color={theme.primary} />
                  </DashboardCard>
                ) : (
                  <DashboardCard>
                    <Text
                      maxFontSizeMultiplier={maxFontSizeMultipliers.content}
                      style={[
                        styles.fallbackTitle,
                        { color: theme.foreground },
                      ]}
                    >
                      {t("agentSessions.composer.setupTitle")}
                    </Text>
                    <Text
                      maxFontSizeMultiplier={maxFontSizeMultipliers.content}
                      style={[
                        styles.fallbackBody,
                        { color: theme.mutedForeground },
                      ]}
                    >
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
            </>
          )}
        </SafeAreaView>
      </SafeAreaProvider>
    </Modal>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  editor: { flex: 1 },
  editorHeader: {
    minHeight: rowMinHeight + space.lg,
    paddingHorizontal: gutter,
    paddingVertical: space.sm,
    flexDirection: "row",
    alignItems: "center",
  },
  headerSide: { width: rowMinHeight + space.md },
  headerRight: { alignItems: "flex-end" },
  submitFeedback: {
    position: "absolute",
    top: rowMinHeight + space.md,
    right: 0,
    zIndex: 10,
    elevation: 10,
  },
  editorTitle: {
    flex: 1,
    paddingHorizontal: space.sm,
    textAlign: "center",
    fontSize: fontSizes.lg,
    fontWeight: fontWeights.medium,
  },
  roundAction: {
    width: rowMinHeight,
    minHeight: rowMinHeight,
    borderRadius: rowMinHeight / 2,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "center",
    justifyContent: "center",
  },
  prompt: {
    flex: 1,
    paddingHorizontal: gutter,
    paddingTop: space.xl,
    paddingBottom: space.md,
    fontSize: fontSizes.xl,
    lineHeight: 27,
    textAlignVertical: "top",
  },
  contextBar: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
    paddingHorizontal: gutter,
    paddingTop: space.sm,
    paddingBottom: space.sm,
  },
  contextChip: {
    minHeight: rowMinHeight,
    borderRadius: rowMinHeight / 2,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: space.md,
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
  },
  repoChip: { flex: 1.4, minWidth: 0 },
  branchChip: { flex: 0.75, minWidth: 76, maxWidth: 110 },
  agentChip: { flex: 0.75, minWidth: 80, maxWidth: 110 },
  chipLabel: {
    flex: 1,
    fontSize: fontSizes.md,
    fontWeight: fontWeights.medium,
  },
  branchInput: {
    flex: 1,
    minWidth: 0,
    paddingVertical: 0,
    fontSize: fontSizes.md,
    fontWeight: fontWeights.medium,
  },
  pressed: { opacity: 0.65 },
  fallbackContent: { padding: gutter, gap: space.md },
  fallbackTitle: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  fallbackBody: {
    fontSize: fontSizes.sm,
    lineHeight: fontSizes.sm * 1.5,
    marginVertical: space.xs,
  },
});
