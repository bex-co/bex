import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { router } from "expo-router";
import {
  ActivityIndicator,
  Linking,
  Modal,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { TopBar } from "@/components/top-bar";
import {
  SafeActionPanel,
  defineSafeAction,
  type MobileActionOption,
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
import { isGitHubUrl } from "../detail/evidence";

const createAgentSession = defineSafeAction(
  "create-agent-session",
  "agent-session",
);

// Only open an https://github.com/ target, never an arbitrary URL a readiness
// payload might smuggle in (shared guard with the detail draft-PR link).
function openGitHub(url: string | null | undefined): void {
  if (isGitHubUrl(url)) void Linking.openURL(url!);
}

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
  const [createdId, setCreatedId] = useState<string | null>(null);
  const set = (patch: Partial<ComposerFields>) =>
    setFields((prev) => ({ ...prev, ...patch }));

  const repoList = useMemo(
    () => (repos.data?.repos ?? []).filter((r) => r?.fullName),
    [repos.data],
  );

  const options: MobileActionOption[] =
    canSubmit({ fields, ready, submitting: false }) && createdId === null
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
                  variables: buildCreateVariables(ownerId, fields),
                });
                const id = result.data?.createAgentSession?.id ?? null;
                if (!id) {
                  return { status: "error", error: new Error("no session id") };
                }
                setCreatedId(id);
                return { status: "accepted_unverified" };
              } catch (error) {
                return { status: "error", error };
              }
            },
          },
        ]
      : [];

  const loadingCaps = capabilities.loading && !capabilities.data;

  return (
    <Modal animationType="slide" onRequestClose={onClose}>
      <SafeAreaView
        style={[styles.safe, { backgroundColor: theme.background }]}
      >
        <TopBar
          title={t("agentSessions.composer.title")}
          right={
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={t("agentSessions.composer.close")}
              onPress={onClose}
              hitSlop={8}
            >
              <Ionicons name="close" size={22} color={theme.foreground} />
            </Pressable>
          }
        />
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
          ) : !enabled || !ready ? (
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
                  accessibilityLabel={t("agentSessions.composer.connectGithub")}
                >
                  {t("agentSessions.composer.connectGithub")}
                </Button>
              ) : null}
            </DashboardCard>
          ) : (
            <>
              <DashboardCard title={t("agentSessions.composer.repo")}>
                {repoList.length === 0 ? (
                  <Text style={[styles.body, { color: theme.mutedForeground }]}>
                    {t("agentSessions.composer.noRepos")}
                  </Text>
                ) : (
                  repoList.map((repo, index) => {
                    const name = repo!.fullName!;
                    const activeRepo = fields.repo === name;
                    return (
                      <Pressable
                        key={name}
                        accessibilityRole="button"
                        onPress={() =>
                          set({
                            repo: name,
                            branch: defaultBranchFor(repo!.defaultBranch),
                          })
                        }
                        style={[
                          styles.pick,
                          index > 0 && {
                            borderTopColor: theme.border,
                            borderTopWidth: StyleSheet.hairlineWidth,
                          },
                        ]}
                      >
                        <Text style={{ color: theme.foreground }}>{name}</Text>
                        {activeRepo ? (
                          <Ionicons
                            name="checkmark"
                            size={18}
                            color={theme.primary}
                          />
                        ) : null}
                      </Pressable>
                    );
                  })
                )}
              </DashboardCard>

              <DashboardCard title={t("agentSessions.composer.branch")}>
                <TextInput
                  value={fields.branch}
                  onChangeText={(branch) => set({ branch })}
                  placeholder="main"
                  placeholderTextColor={theme.mutedForeground}
                  autoCapitalize="none"
                  autoCorrect={false}
                  style={[
                    styles.input,
                    { color: theme.foreground, borderColor: theme.border },
                  ]}
                />
              </DashboardCard>

              <DashboardCard title={t("agentSessions.composer.agent")}>
                {agents.map((agent, index) => {
                  const activeAgent = fields.agent === agent.id;
                  return (
                    <Pressable
                      key={agent.id}
                      accessibilityRole="button"
                      onPress={() => set({ agent: agent.id })}
                      style={[
                        styles.pick,
                        index > 0 && {
                          borderTopColor: theme.border,
                          borderTopWidth: StyleSheet.hairlineWidth,
                        },
                      ]}
                    >
                      <Text style={{ color: theme.foreground }}>
                        {agent.label}
                      </Text>
                      {activeAgent ? (
                        <Ionicons
                          name="checkmark"
                          size={18}
                          color={theme.primary}
                        />
                      ) : null}
                    </Pressable>
                  );
                })}
              </DashboardCard>

              <DashboardCard title={t("agentSessions.composer.prompt")}>
                <TextInput
                  value={fields.prompt}
                  onChangeText={(prompt) => set({ prompt })}
                  placeholder={t("agentSessions.composer.promptPlaceholder")}
                  placeholderTextColor={theme.mutedForeground}
                  multiline
                  style={[
                    styles.input,
                    styles.multiline,
                    { color: theme.foreground, borderColor: theme.border },
                  ]}
                />
              </DashboardCard>

              <SafeActionPanel options={options} />
            </>
          )}
        </ScrollView>
      </SafeAreaView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { padding: gutter, gap: space.md },
  title: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  body: {
    fontSize: fontSizes.sm,
    lineHeight: fontSizes.sm * 1.5,
    marginVertical: space.xs,
  },
  pick: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: space.md,
  },
  input: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 8,
    paddingHorizontal: space.sm,
    paddingVertical: space.sm,
    fontSize: fontSizes.md,
  },
  multiline: { minHeight: 96, textAlignVertical: "top" },
});
