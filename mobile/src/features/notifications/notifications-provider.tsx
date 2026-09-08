import AsyncStorage from "@react-native-async-storage/async-storage";
import * as SecureStore from "expo-secure-store";
import { randomUUID } from "expo-crypto";
import * as Notifications from "expo-notifications";
import { useRouter } from "expo-router";
import { Platform } from "react-native";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { dataBoundary } from "@/common/apollo/data-boundary";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
import { notificationActions } from "./destination-access";
import { PendingNotificationResponse } from "./pending-response";
import { apolloClient } from "@/common/apollo/apollo-client";
import { useNetworkState } from "@/common/apollo/network-state";
import { authManager, useAuth } from "@/features/auth/auth-provider";
import { mobileConfig } from "@/features/auth/config";
import {
  notificationMatchesBinding,
  parseNotificationEnvelope,
  type NotificationEnvelope,
} from "./deep-link";
import { ExpoNotificationAdapter } from "./expo-adapter";
import { ApolloNotificationSubscriptionClient } from "./graphql-client";
import {
  NotificationInboxStore,
  type NotificationInboxItem,
} from "./inbox-store";
import { NotificationInstallationStore } from "./installation-store";
import { NotificationRegistrationPreference } from "./preferences";
import {
  NotificationRegistrationController,
  type NotificationRegistrationState,
} from "./registration-controller";
import { useWorkspace } from "@/features/workspaces/workspace-provider";

type NotificationsContextValue = {
  state: NotificationRegistrationState;
  items: NotificationInboxItem[];
  unread: number;
  inboxState: "checking" | "ready" | "error";
  retry: () => Promise<void>;
  enable: () => Promise<void>;
  disable: () => Promise<void>;
  markAllRead: () => Promise<void>;
  open: (item: NotificationInboxItem) => Promise<void>;
};

const NotificationsContext = createContext<NotificationsContextValue | null>(
  null,
);
const native = new ExpoNotificationAdapter();
const installation = new NotificationInstallationStore(SecureStore, randomUUID);
const preference = new NotificationRegistrationPreference(AsyncStorage);
// Badge writes are process-wide. A delayed native setter must finish before a
// new workspace's count is applied, and queued obsolete writes are skipped.
let badgeQueue: Promise<unknown> = Promise.resolve();
function writeBadge(count: number, current: () => boolean = () => true) {
  badgeQueue = badgeQueue
    .catch(() => undefined)
    .then(() => {
      if (current()) return Notifications.setBadgeCountAsync(count);
      return undefined;
    });
  return badgeQueue;
}
let mayPresent: ((envelope: NotificationEnvelope) => boolean) | null = null;
Notifications.setNotificationHandler({
  handleNotification: async (notification) => {
    const envelope = parseNotificationEnvelope(
      notification.request.content.data,
    );
    const allowed = envelope !== null && mayPresent?.(envelope) === true;
    return {
      shouldShowBanner: allowed,
      shouldShowList: allowed,
      shouldPlaySound: allowed,
      shouldSetBadge: false,
    };
  },
});

function createNotificationScope(
  binding: { subject: string; sessionId: string; workspaceId: string },
  generation: number,
  live: RefObject<{
    workspace: ReturnType<typeof useWorkspace>;
    access: ReturnType<typeof useCapabilities>;
    network: ReturnType<typeof useNetworkState>;
  }>,
  mounted: RefObject<boolean>,
  signingOut: RefObject<boolean>,
) {
  const { subject, sessionId, workspaceId } = binding;
  const current = () => {
    const identity = authManager.getState();
    return (
      mounted.current &&
      !signingOut.current &&
      identity.status === "signedIn" &&
      identity.session.subject === subject &&
      identity.session.sessionId === sessionId &&
      live.current.workspace.selected?.id === workspaceId &&
      dataBoundary.workspaceId === workspaceId &&
      dataBoundary.getGeneration() === generation
    );
  };
  const ready = () =>
    current() &&
    live.current.workspace.status === "ready" &&
    !live.current.workspace.switching &&
    live.current.access.allows("can_view");
  const allowed = (item: { event: string; route: string }) =>
    ready() &&
    notificationActions(item).every((action) =>
      live.current.access.allows(action),
    );
  const denied = (item: { event: string; route: string }) =>
    notificationActions(item).some((action) =>
      live.current.access.denied(action),
    );
  const canOpen = (envelope: NotificationEnvelope) =>
    notificationMatchesBinding(envelope, binding) && allowed(envelope);
  return {
    binding,
    current,
    ready,
    allowed,
    canOpen,
    denied,
    subscriptions: new ApolloNotificationSubscriptionClient(
      apolloClient,
      workspaceId,
    ),
    store: new NotificationInboxStore(
      AsyncStorage,
      `bex.notifications.inbox.v1:${sessionId}:${workspaceId}`,
      100,
      current,
    ),
  };
}

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const { state: auth } = useAuth();
  const workspace = useWorkspace();
  const access = useCapabilities();
  const network = useNetworkState();
  const router = useRouter();
  const sessionId =
    auth.status === "signedIn" ? auth.session.sessionId : "signed-out";
  const subject =
    auth.status === "signedIn" ? auth.session.subject : "signed-out";
  const workspaceId = workspace.selected?.id ?? "no-workspace";
  const generation = access.generation;
  const live = useRef({ workspace, access, network });
  live.current = { workspace, access, network };
  const signingOut = useRef(false);
  const mounted = useRef(true);
  const pending = useRef(
    new PendingNotificationResponse<Notifications.Notification>(),
  );
  const [state, setState] = useState<NotificationRegistrationState>("checking");
  const scope = useMemo(
    () =>
      createNotificationScope(
        { subject, sessionId, workspaceId },
        generation,
        live,
        mounted,
        signingOut,
      ),
    [subject, sessionId, workspaceId, generation],
  );
  const [inboxResult, setInboxResult] = useState<{
    scope: typeof scope;
    state: "checking" | "ready" | "error";
  } | null>(null);
  const [local, setLocal] = useState<{
    scope: typeof scope;
    items: NotificationInboxItem[];
  } | null>(null);
  // Filter during render, not after an effect: no old title can appear during
  // a switch or access reset. The same live policy guards every callback.
  const items = local?.scope === scope ? local.items.filter(scope.allowed) : [];
  const ready = scope.ready();
  const unread = items.filter((item) => !item.read).length;
  useLayoutEffect(() => {
    void writeBadge(unread, scope.current);
  }, [scope, unread, ready]);
  const registration = useMemo(
    () => ({ current: null as NotificationRegistrationController | null }),
    [scope],
  );

  useLayoutEffect(() => {
    mounted.current = true;
    const present = scope.canOpen;
    mayPresent = present;
    return () => {
      if (mayPresent === present) mayPresent = null;
    };
  }, [scope]);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const receive = useCallback(
    async (
      notification: Notifications.Notification,
      tapped: boolean,
    ): Promise<boolean> => {
      const envelope = parseNotificationEnvelope(
        notification.request.content.data,
      );
      if (!envelope || !scope.canOpen(envelope)) return false;
      let next = await scope.store.record(envelope, {
        title: notification.request.content.title,
        body: notification.request.content.body,
        receivedAt: notification.date,
      });
      if (!scope.canOpen(envelope)) return false;
      if (tapped) {
        next = await scope.store.markRead(envelope.notificationId);
        if (!scope.canOpen(envelope)) return false;
      }
      setLocal({ scope, items: next });
      if (tapped) {
        void scope.subscriptions
          .markNotificationRead(envelope.notificationId)
          .catch(() => undefined);
        router.push(envelope.route);
      }
      return true;
    },
    [scope, router],
  );

  const drain = useCallback(
    () =>
      pending.current
        .drain(
          (notification) => {
            const envelope = parseNotificationEnvelope(
              notification.request.content.data,
            );
            if (!envelope) return "reject";
            if (
              !scope.current() ||
              live.current.workspace.status !== "ready" ||
              live.current.workspace.switching
            )
              return "wait";
            if (!notificationMatchesBinding(envelope, scope.binding))
              return "reject";
            if (
              notificationActions(envelope).some((action) =>
                live.current.access.denied(action),
              )
            )
              return "reject";
            return scope.canOpen(envelope) ? "open" : "wait";
          },
          (notification) => receive(notification, true),
          (id) => {
            // A new OS response may have arrived while this one was being processed.
            if (
              Notifications.getLastNotificationResponse()?.notification.request
                .identifier === id
            ) {
              Notifications.clearLastNotificationResponse();
            }
          },
        )
        .catch(() => undefined),
    [scope, receive],
  );

  useEffect(() => {
    const capture = (response: Notifications.NotificationResponse) => {
      // The OS retains the initial response until this scope becomes current.
      // A removed listener must not replace a newer scope's pending tap.
      if (!scope.current()) return;
      pending.current.capture(
        response.notification.request.identifier,
        response.notification,
      );
      void drain();
    };
    const received = Notifications.addNotificationReceivedListener(
      (notification) => {
        void receive(notification, false).catch(() => undefined);
      },
    );
    const tapped =
      Notifications.addNotificationResponseReceivedListener(capture);
    const initial = Notifications.getLastNotificationResponse();
    if (initial) capture(initial);
    void drain();
    return () => {
      received.remove();
      tapped.remove();
    };
  }, [drain, receive, scope]);

  useEffect(() => {
    void drain();
  }, [drain, ready, access.state]);

  useEffect(() => {
    if (!ready) {
      void writeBadge(0);
      if (scope.current()) {
        void scope.store
          .reconcile((item) => !scope.denied(item))
          .catch(() => undefined);
      }
      return;
    }
    let active = true;
    const current = () => active && scope.ready();
    const report = (state: "checking" | "ready" | "error") => {
      if (current()) setInboxResult({ scope, state });
    };
    report("checking");
    void scope.store
      .reconcile((item) => !scope.denied(item))
      .then(async (cached) => {
        if (!current()) return;
        setLocal({ scope, items: cached });
        try {
          const remote = await scope.subscriptions.inbox();
          if (!current()) return;
          const next = await scope.store.mergeRemote(
            remote.filter(scope.allowed),
          );
          if (current()) setLocal({ scope, items: next });
          report("ready");
        } catch {
          report("error");
          // A failed read is not an authoritative empty inbox.
        }
      })
      .catch(() => report("error"));
    return () => {
      active = false;
    };
  }, [scope, ready, access.state]);

  useEffect(() => {
    const controller = new NotificationRegistrationController(
      mobileConfig.easProjectId ?? null,
      Platform.OS === "android" ? "android" : "ios",
      native,
      scope.subscriptions,
      () => installation.getOrCreate(),
      preference,
      () => live.current.network === "online",
      scope.ready,
      (next) => {
        if (scope.current()) setState(next);
      },
      () => scope.binding,
    );
    registration.current = controller;
    if (ready)
      void controller.inspectAndRepair().catch(() => {
        if (scope.current()) setState("error");
      });
    const rotated = Notifications.addPushTokenListener(() => {
      if (scope.ready()) void controller.repairAfterTokenRotation();
    });
    const removeClearHook = authManager.registerSessionClearHook(
      async (session) => {
        signingOut.current = true;
        mayPresent = null;
        controller.dispose();
        setLocal(null);
        void writeBadge(0);
        await scope.store.clear();
        if (session) await controller.unregisterCurrent(session.accessToken);
      },
    );
    return () => {
      controller.dispose();
      if (registration.current === controller) registration.current = null;
      rotated.remove();
      removeClearHook();
    };
  }, [registration, network, ready, scope]);

  const enable = async () => {
    if (scope.ready()) await registration.current?.enableFromUserGesture();
  };
  const disable = async () => {
    if (scope.ready())
      await registration.current?.unregisterCurrent().catch(() => {
        if (scope.current()) setState("error");
      });
  };
  const markAllRead = async () => {
    if (!scope.ready()) return;
    const ids = items.filter((item) => !item.read).map((item) => item.id);
    const next = await scope.store.markAllRead();
    if (!scope.ready()) return;
    setLocal({ scope, items: next });
    await Promise.allSettled(
      ids.map((id) => scope.subscriptions.markNotificationRead(id)),
    );
  };
  const open = async (item: NotificationInboxItem) => {
    // Require an item from this rendered snapshot, not a reconstructed envelope
    // that could turn a stale A-workspace row into a B-workspace notification.
    if (!items.includes(item) || !scope.allowed(item)) return;
    const next = await scope.store.markRead(item.id);
    if (!scope.allowed(item)) return;
    setLocal({ scope, items: next });
    void scope.subscriptions
      .markNotificationRead(item.id)
      .catch(() => undefined);
    router.push(item.route);
  };
  return (
    <NotificationsContext.Provider
      value={{
        state: ready ? state : "checking",
        inboxState: !ready
          ? access.state.status === "unavailable" || access.offline
            ? "error"
            : "checking"
          : inboxResult?.scope === scope
            ? inboxResult.state
            : "checking",
        retry: async () => {
          await access.retry();
        },
        items,
        unread,
        enable,
        disable,
        markAllRead,
        open,
      }}
    >
      {children}
    </NotificationsContext.Provider>
  );
}

export function useNotifications(): NotificationsContextValue {
  const value = useContext(NotificationsContext);
  if (!value)
    throw new Error(
      "useNotifications must be used inside NotificationsProvider",
    );
  return value;
}
