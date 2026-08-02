import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import {
  ActivityIndicator,
  AppState,
  Keyboard,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { dataBoundary } from "@/common/apollo/data-boundary";
import { useTranslations } from "@/common/hooks/use-translations";
import { fonts, fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { SafeActionConfirmationDialog } from "@/components/safe-action/safe-action-dialog";
import {
  MobileEnvVarKeysDocument,
  MobilePatchSingleEnvVarDocument,
  MobileRevealEnvVarDocument,
} from "@/generated-graphql";
import {
  classifyEnvironmentFailure,
  type EnvironmentFailure,
} from "./environment-errors";
import { parseMaskedEnvironmentList } from "./environment-masked-list";
import {
  EnvironmentOperationGuard,
  environmentTimeoutError,
} from "./environment-operation-guard";
import {
  confirmEnvironmentEditIntent,
  createEnvironmentEditIntent,
  EnvironmentSecretSession,
  type EnvironmentEditIntent,
} from "./environment-intent";

type FeedbackKind =
  | "saved"
  | "saved-no-rollout"
  | "committed-refresh-unavailable"
  | "reveal-unavailable"
  | EnvironmentFailure;
type Feedback = { kind: FeedbackKind } | null;

const SECRET_OPERATION_TIMEOUT_MS = 15_000;

export type EnvironmentCardHandle = {
  refresh: () => Promise<void>;
};

export const EnvironmentCard = forwardRef<
  EnvironmentCardHandle,
  {
    serviceId: string;
    serviceLabel: string;
  }
>(function EnvironmentCard({ serviceId, serviceLabel }, ref) {
  /*
   * This stays embedded in service detail: mobile deliberately has no broad
   * environment-management route or creation surface.
   */
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const client = useApolloClient();
  const session = useRef(new EnvironmentSecretSession()).current;
  const operations = useRef(new EnvironmentOperationGuard()).current;
  const serviceIdRef = useRef(serviceId);
  serviceIdRef.current = serviceId;
  const mountedRef = useRef(true);
  const commitAwaitingRefreshRef = useRef(false);
  const [revealed, setRevealed] = useState(session.value());
  const [revealLoading, setRevealLoading] = useState<string | null>(null);
  const [intent, setIntent] = useState<EnvironmentEditIntent | null>(null);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const [maskedRefreshRequired, setMaskedRefreshRequired] = useState(false);
  const [refreshingMasked, setRefreshingMasked] = useState(false);
  const savingRef = useRef(false);
  const [saving, setSaving] = useState(false);
  const listQuery = useQuery(MobileEnvVarKeysDocument, {
    variables: { serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
  });
  const [patchOne] = useMutation(MobilePatchSingleEnvVarDocument);

  const maskedList = useMemo(
    () =>
      listQuery.data &&
      (listQuery.data.service == null ||
        listQuery.data.service.id !== serviceId)
        ? ({ valid: false, variables: [] } as const)
        : parseMaskedEnvironmentList(listQuery.data?.service?.envVarKeys ?? []),
    [listQuery.data, serviceId],
  );
  const variables = maskedList.variables;
  const visibleReveal = revealed?.serviceId === serviceId ? revealed : null;
  const visibleIntent = intent?.serviceId === serviceId ? intent : null;

  const refreshMaskedList = useCallback(async (): Promise<boolean> => {
    const requestedService = serviceId;
    const lease = operations.begin("refresh", SECRET_OPERATION_TIMEOUT_MS);
    try {
      const result = await client.query({
        query: MobileEnvVarKeysDocument,
        variables: { serviceId: requestedService },
        fetchPolicy: "network-only",
        errorPolicy: "none",
        context: { fetchOptions: { signal: lease.signal } },
      });
      if (lease.status() === "timed-out") throw environmentTimeoutError();
      const refreshed = parseMaskedEnvironmentList(
        result.data?.service?.envVarKeys ?? [],
      );
      if (!lease.isCurrent() || serviceIdRef.current !== requestedService) {
        return false;
      }
      if (result.data?.service?.id !== requestedService || !refreshed.valid) {
        setMaskedRefreshRequired(true);
        return false;
      }
      commitAwaitingRefreshRef.current = false;
      setMaskedRefreshRequired(false);
      return true;
    } catch {
      if (
        lease.isCurrent() &&
        serviceIdRef.current === requestedService &&
        mountedRef.current
      ) {
        setMaskedRefreshRequired(true);
      }
      return false;
    } finally {
      lease.finish();
    }
  }, [client, operations, serviceId]);

  useImperativeHandle(
    ref,
    () => ({
      refresh: async () => {
        session.clear();
        setIntent(null);
        if (!(await refreshMaskedList())) {
          throw new Error("masked environment refresh failed");
        }
        setFeedback(null);
      },
    }),
    [refreshMaskedList, session],
  );

  useEffect(() => session.subscribe(setRevealed), [session]);
  useEffect(() => {
    operations.invalidate();
    session.clear();
    setIntent(null);
    setFeedback(null);
    setRevealLoading(null);
    setSaving(false);
    savingRef.current = false;
    commitAwaitingRefreshRef.current = false;
    setMaskedRefreshRequired(false);
    Keyboard.dismiss();
  }, [operations, serviceId, session]);
  useEffect(
    () =>
      dataBoundary.registerResetHandler(() => {
        operations.invalidate();
        session.clear();
        setIntent(null);
        setFeedback(null);
        setRevealLoading(null);
        setSaving(false);
        savingRef.current = false;
        commitAwaitingRefreshRef.current = false;
        setMaskedRefreshRequired(false);
        Keyboard.dismiss();
      }),
    [operations, session],
  );
  useEffect(() => {
    const subscription = AppState.addEventListener("change", (state) => {
      if (state !== "active") {
        const mutationUnknown = operations.hasActive("mutation");
        const committed = commitAwaitingRefreshRef.current;
        operations.invalidate();
        session.clear();
        setIntent(null);
        setRevealLoading(null);
        setSaving(false);
        savingRef.current = false;
        Keyboard.dismiss();
        if (mutationUnknown || committed) {
          setMaskedRefreshRequired(true);
          setFeedback({
            kind: committed
              ? "committed-refresh-unavailable"
              : "timeout-unknown",
          });
        }
      }
    });
    return () => subscription.remove();
  }, [operations, session]);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      operations.invalidate();
      session.clear();
      Keyboard.dismiss();
    };
  }, [operations, session]);
  useEffect(() => {
    if (
      visibleReveal &&
      !variables.some((item) => item.key === visibleReveal.key)
    ) {
      session.clear();
      setIntent(null);
    }
  }, [session, variables, visibleReveal]);

  async function reveal(key: string) {
    if (
      revealLoading ||
      savingRef.current ||
      maskedRefreshRequired ||
      !maskedList.valid ||
      !variables.some((item) => item.key === key)
    ) {
      return;
    }
    const requestedService = serviceId;
    const lease = operations.begin("reveal", SECRET_OPERATION_TIMEOUT_MS);
    session.clear();
    setIntent(null);
    setFeedback(null);
    setRevealLoading(key);
    try {
      const result = await client.query({
        query: MobileRevealEnvVarDocument,
        variables: { serviceId, key },
        fetchPolicy: "no-cache",
        errorPolicy: "none",
        context: { fetchOptions: { signal: lease.signal } },
      });
      if (lease.status() === "timed-out") throw environmentTimeoutError();
      const value = result.data?.service?.envVar;
      if (
        !lease.isCurrent() ||
        AppState.currentState !== "active" ||
        serviceIdRef.current !== requestedService ||
        result.data?.service?.id !== requestedService ||
        value?.key !== key ||
        typeof value.value !== "string" ||
        !value.revision
      ) {
        throw Object.assign(new Error("environment value was not found"), {
          code: "NOT_FOUND",
        });
      }
      session.reveal({
        serviceId: requestedService,
        key,
        value: value.value,
        revision: value.revision,
      });
    } catch (error) {
      if (
        !lease.isCurrent() ||
        serviceIdRef.current !== requestedService ||
        !mountedRef.current
      ) {
        return;
      }
      const kind = classifyEnvironmentFailure(error);
      setFeedback({
        kind: kind === "timeout-unknown" ? "reveal-unavailable" : kind,
      });
      if (kind === "not-found" || kind === "revision-conflict") {
        setMaskedRefreshRequired(true);
        await refreshMaskedList();
      }
    } finally {
      lease.finish();
      if (
        lease.isCurrent() &&
        serviceIdRef.current === requestedService &&
        mountedRef.current
      ) {
        setRevealLoading(null);
      }
    }
  }

  function requestSave() {
    if (!visibleReveal || savingRef.current) return;
    const nextIntent = createEnvironmentEditIntent(visibleReveal, serviceLabel);
    Keyboard.dismiss();
    session.clear();
    setFeedback(null);
    setIntent(nextIntent);
  }

  async function confirmSave() {
    if (
      !visibleIntent ||
      savingRef.current ||
      maskedRefreshRequired ||
      !maskedList.valid
    ) {
      return;
    }
    const requestedService = serviceId;
    const confirmed = confirmEnvironmentEditIntent(visibleIntent);
    setIntent(confirmed);
    savingRef.current = true;
    setSaving(true);
    const lease = operations.begin("mutation", SECRET_OPERATION_TIMEOUT_MS);
    let leaseFinished = false;
    try {
      const result = await patchOne({
        variables: {
          serviceId: confirmed.serviceId,
          key: confirmed.key,
          value: confirmed.value,
          revision: confirmed.revision,
        },
        context: {
          fetchOptions: { signal: lease.signal },
          skipAuthRefresh: true,
        },
      });
      if (lease.status() === "timed-out") throw environmentTimeoutError();
      if (
        !lease.isCurrent() ||
        serviceIdRef.current !== requestedService ||
        !mountedRef.current
      ) {
        return;
      }
      if (!result.data?.patchServiceEnvironment) {
        throw new Error("environment update returned no result");
      }
      const rolledOut = result.data.patchServiceEnvironment.rolledOut;
      commitAwaitingRefreshRef.current = true;
      lease.finish();
      leaseFinished = true;
      setIntent(null);
      const refreshed = await refreshMaskedList();
      if (
        serviceIdRef.current !== requestedService ||
        !mountedRef.current ||
        AppState.currentState !== "active"
      ) {
        return;
      }
      setFeedback({
        kind: refreshed
          ? rolledOut
            ? "saved"
            : "saved-no-rollout"
          : "committed-refresh-unavailable",
      });
    } catch (error) {
      if (
        !lease.isCurrent() ||
        serviceIdRef.current !== requestedService ||
        !mountedRef.current
      ) {
        return;
      }
      const kind = classifyEnvironmentFailure(error);
      setIntent(null);
      setFeedback({ kind });
      setMaskedRefreshRequired(true);
      await refreshMaskedList();
    } finally {
      if (!leaseFinished) lease.finish();
      if (serviceIdRef.current === requestedService && mountedRef.current) {
        savingRef.current = false;
        setSaving(false);
      }
    }
  }

  async function reconcileMaskedState() {
    if (refreshingMasked) return;
    operations.invalidate();
    session.clear();
    setIntent(null);
    setRevealLoading(null);
    Keyboard.dismiss();
    setRefreshingMasked(true);
    try {
      if (await refreshMaskedList()) setFeedback(null);
    } finally {
      if (mountedRef.current && serviceIdRef.current === serviceId) {
        setRefreshingMasked(false);
      }
    }
  }

  const listFailure = listQuery.error
    ? classifyEnvironmentFailure(listQuery.error)
    : !maskedList.valid
      ? "revision-unavailable"
      : null;
  const reconciliationNeeded =
    maskedRefreshRequired || !maskedList.valid || Boolean(listQuery.error);

  return (
    <DashboardCard title={t("environment.title")}>
      <Text style={[styles.description, { color: theme.mutedForeground }]}>
        {t("environment.description")}
      </Text>

      {listFailure ? (
        <EnvironmentNotice kind={listFailure} />
      ) : listQuery.loading && variables.length === 0 ? (
        <ActivityIndicator
          accessibilityLabel={t("environment.loading")}
          color={theme.primary}
          style={styles.loader}
        />
      ) : variables.length === 0 ? (
        <Text style={{ color: theme.mutedForeground }}>
          {t("environment.empty")}
        </Text>
      ) : (
        <View style={styles.list}>
          {variables.map((item) => {
            const open = visibleReveal?.key === item.key;
            return (
              <View
                key={item.id}
                style={[styles.row, { borderTopColor: theme.border }]}
              >
                <View style={styles.rowHeader}>
                  <View style={styles.keyCopy}>
                    <Text
                      numberOfLines={1}
                      style={[styles.key, { color: theme.foreground }]}
                    >
                      {item.key}
                    </Text>
                    {!open ? (
                      <Text
                        accessibilityLabel={t("environment.masked")}
                        style={[
                          styles.masked,
                          { color: theme.mutedForeground },
                        ]}
                      >
                        ••••••••
                      </Text>
                    ) : null}
                  </View>
                  <Pressable
                    accessibilityRole="button"
                    accessibilityLabel={
                      open
                        ? t("environment.hideKey", { key: item.key })
                        : t("environment.revealKey", { key: item.key })
                    }
                    disabled={
                      Boolean(revealLoading) || saving || reconciliationNeeded
                    }
                    onPress={() =>
                      open ? session.clear() : void reveal(item.key)
                    }
                    style={styles.revealButton}
                  >
                    {revealLoading === item.key ? (
                      <ActivityIndicator color={theme.primary} />
                    ) : (
                      <Text style={{ color: theme.primary }}>
                        {open ? t("environment.hide") : t("environment.reveal")}
                      </Text>
                    )}
                  </Pressable>
                </View>
                {open ? (
                  <View style={styles.editor}>
                    <TextInput
                      accessibilityLabel={t("environment.valueFor", {
                        key: item.key,
                      })}
                      autoCapitalize="none"
                      autoComplete="off"
                      autoCorrect={false}
                      multiline
                      spellCheck={false}
                      value={visibleReveal.value}
                      onChangeText={(value) => session.edit(value)}
                      editable={!saving}
                      style={[
                        styles.input,
                        {
                          color: theme.foreground,
                          borderColor: theme.border,
                          backgroundColor: theme.card,
                        },
                      ]}
                    />
                    <View style={styles.editorButtons}>
                      <Button
                        type="outline"
                        style={styles.editorButton}
                        disabled={saving}
                        onPress={() => session.clear()}
                        accessibilityLabel={t("environment.cancelEdit")}
                      >
                        {t("common.cancel")}
                      </Button>
                      <Button
                        style={styles.editorButton}
                        loading={saving}
                        disabled={saving}
                        onPress={requestSave}
                        accessibilityLabel={t("environment.reviewSave", {
                          key: item.key,
                        })}
                      >
                        {t("environment.review")}
                      </Button>
                    </View>
                  </View>
                ) : null}
              </View>
            );
          })}
        </View>
      )}

      {reconciliationNeeded ? (
        <View
          accessibilityRole="alert"
          style={[styles.notice, { borderColor: theme.warning }]}
        >
          <Text style={{ color: theme.warning }}>
            {t("environment.refreshRequired")}
          </Text>
          <Button
            type="outline"
            style={styles.refreshButton}
            loading={refreshingMasked}
            disabled={refreshingMasked || saving}
            onPress={() => void reconcileMaskedState()}
            accessibilityLabel={t("environment.refresh")}
          >
            {refreshingMasked
              ? t("environment.refreshing")
              : t("environment.refresh")}
          </Button>
        </View>
      ) : null}

      {feedback ? <EnvironmentFeedback kind={feedback.kind} /> : null}

      <SafeActionConfirmationDialog
        intent={visibleIntent?.action ?? null}
        pending={saving}
        title={t("environment.confirmTitle")}
        message={t("environment.confirmMessage")}
        actionLabel={t("environment.updateAction")}
        confirmLabel={t("environment.confirm")}
        cancelLabel={t("common.cancel")}
        pendingLabel={t("environment.saving")}
        onConfirm={() => void confirmSave()}
        onCancel={() => setIntent(null)}
      />
    </DashboardCard>
  );
});

function EnvironmentNotice({ kind }: { kind: EnvironmentFailure }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View
      accessibilityRole="alert"
      style={[styles.notice, { borderColor: theme.warning }]}
    >
      <Text style={{ color: theme.warning }}>
        {t(`environment.errors.${kind}`)}
      </Text>
    </View>
  );
}

function EnvironmentFeedback({ kind }: { kind: FeedbackKind }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  if (
    kind !== "saved" &&
    kind !== "saved-no-rollout" &&
    kind !== "committed-refresh-unavailable" &&
    kind !== "reveal-unavailable"
  ) {
    return <EnvironmentNotice kind={kind} />;
  }
  const success = kind === "saved" || kind === "saved-no-rollout";
  const message =
    kind === "saved"
      ? t("environment.saved")
      : kind === "saved-no-rollout"
        ? t("environment.savedNoRollout")
        : kind === "committed-refresh-unavailable"
          ? t("environment.committedRefreshUnavailable")
          : t("environment.revealUnavailable");
  return (
    <View
      accessibilityRole="alert"
      style={[
        styles.notice,
        { borderColor: success ? theme.success : theme.warning },
      ]}
    >
      <Text style={{ color: success ? theme.success : theme.warning }}>
        {message}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  description: { fontSize: fontSizes.sm, marginBottom: space.md },
  loader: { minHeight: 64 },
  list: { gap: 0 },
  row: { borderTopWidth: StyleSheet.hairlineWidth, paddingVertical: space.md },
  rowHeader: {
    minHeight: 44,
    flexDirection: "row",
    alignItems: "center",
    gap: space.md,
  },
  keyCopy: { flex: 1, gap: space.xs },
  key: {
    fontFamily: fonts.monoMedium,
    fontSize: fontSizes.md,
    fontWeight: fontWeights.medium,
  },
  masked: { fontFamily: fonts.mono, letterSpacing: 1 },
  revealButton: {
    minWidth: 72,
    minHeight: 44,
    alignItems: "flex-end",
    justifyContent: "center",
  },
  editor: { gap: space.sm, paddingTop: space.sm },
  input: {
    minHeight: 96,
    borderWidth: 1,
    borderRadius: space.sm,
    padding: space.md,
    fontFamily: fonts.mono,
    fontSize: fontSizes.md,
    textAlignVertical: "top",
  },
  editorButtons: { flexDirection: "row", gap: space.sm },
  editorButton: { flex: 1, paddingHorizontal: space.sm },
  notice: {
    borderWidth: 1,
    borderRadius: space.sm,
    padding: space.md,
    marginTop: space.md,
  },
  refreshButton: { marginTop: space.sm, paddingHorizontal: space.sm },
});
