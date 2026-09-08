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
  useRef,
  useState,
  type ReactNode,
} from "react";
import { apolloClient } from "@/common/apollo/apollo-client";
import { useNetworkState } from "@/common/apollo/network-state";
import { authManager, useAuth } from "@/features/auth/auth-provider";
import { mobileConfig } from "@/features/auth/config";
import {
  notificationMatchesBinding,
  parseNotificationEnvelope,
  type NotificationRoute,
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
  type NotificationBinding,
  type NotificationRegistrationState,
} from "./registration-controller";
import { useWorkspace } from "@/features/workspaces/workspace-provider";

type NotificationsContextValue = {
  state: NotificationRegistrationState;
  items: NotificationInboxItem[];
  unread: number;
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
let activeNotificationBinding: NotificationBinding | null = null;

function currentNotificationBinding(): NotificationBinding | null {
  return activeNotificationBinding;
}

function publishNotificationBinding(binding: NotificationBinding): void {
  activeNotificationBinding = binding;
}

function clearNotificationBinding(sessionId?: string): void {
  if (
    sessionId === undefined ||
    activeNotificationBinding?.sessionId === sessionId
  ) {
    activeNotificationBinding = null;
  }
}

// codex-security round-7 F9: OS-level presentation is gated on a live session.
// Logout's remote push unregistration is deliberately fire-and-forget (the
// runbook documents it must not make logout depend on the network), so a failed
// unregister leaves the server subscription active and the OS keeps delivering
// — without this gate the prior account's deploy events would banner on a
// signed-out or account-switched device. authManager is a module singleton
// with synchronous state; the cold-start "loading" phase fails closed (no
// banners until a session is confirmed). In-app handling was already gated.
Notifications.setNotificationHandler({
  handleNotification: async (notification) => {
    const envelope = parseNotificationEnvelope(
      notification.request.content.data,
    );
    const auth = authManager.getState();
    const activeBinding = currentNotificationBinding();
    const binding =
      auth.status === "signedIn" &&
      activeBinding?.subject === auth.session.subject &&
      activeBinding.sessionId === auth.session.sessionId
        ? activeBinding
        : null;
    const signedIn =
      envelope !== null && notificationMatchesBinding(envelope, binding);
    return {
      shouldShowBanner: signedIn,
      shouldShowList: signedIn,
      shouldPlaySound: signedIn,
      shouldSetBadge: false,
    };
  },
});

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const { state: auth } = useAuth();
  const { selected } = useWorkspace();
  const network = useNetworkState();
  const router = useRouter();
  const signedIn = auth.status === "signedIn";
  const sessionId = signedIn ? auth.session.sessionId : "signed-out";
  const subject = signedIn ? auth.session.subject : "signed-out";
  const workspaceId = selected?.id ?? "no-workspace";
  const subscriptions = useMemo(
    () => new ApolloNotificationSubscriptionClient(apolloClient, workspaceId),
    [workspaceId],
  );
  const activeSessionRef = useRef(sessionId);
  const signingOutRef = useRef(false);
  activeSessionRef.current = sessionId;
  const [state, setState] = useState<NotificationRegistrationState>("checking");
  const [items, setItems] = useState<NotificationInboxItem[]>([]);
  const store = useMemo(
    () =>
      new NotificationInboxStore(
        AsyncStorage,
        { set: (count) => Notifications.setBadgeCountAsync(count) },
        `bex.notifications.inbox.v1:${sessionId}:${workspaceId}`,
      ),
    [sessionId, workspaceId],
  );
  const storeRef = useRef(store);
  storeRef.current = store;
  const controller = useMemo(
    () =>
      new NotificationRegistrationController(
        mobileConfig.easProjectId ?? null,
        Platform.OS === "android" ? "android" : "ios",
        native,
        subscriptions,
        () => installation.getOrCreate(),
        preference,
        () => network === "online",
        () => signedIn && selected !== null,
        setState,
        () => ({ subject, sessionId, workspaceId }),
        (binding) => {
          if (
            authManager.getState().status === "signedIn" &&
            activeSessionRef.current === binding.sessionId
          ) {
            publishNotificationBinding(binding);
          }
        },
      ),
    [
      network,
      selected,
      sessionId,
      signedIn,
      subject,
      workspaceId,
      subscriptions,
    ],
  );
  const controllerRef = useRef(controller);
  controllerRef.current = controller;

  const receive = useCallback(
    async (notification: Notifications.Notification, tapped: boolean) => {
      const envelope = parseNotificationEnvelope(
        notification.request.content.data,
      );
      if (
        !envelope ||
        !notificationMatchesBinding(envelope, currentNotificationBinding()) ||
        !signedIn ||
        signingOutRef.current
      )
        return;
      const receivingSession = sessionId;
      const next = await store.record(envelope, {
        title: notification.request.content.title,
        body: notification.request.content.body,
        receivedAt: notification.date,
      });
      if (
        signingOutRef.current ||
        activeSessionRef.current !== receivingSession
      ) {
        return;
      }
      if (!tapped) return setItems(next);
      const read = await store.markRead(envelope.notificationId);
      if (
        signingOutRef.current ||
        activeSessionRef.current !== receivingSession
      ) {
        return;
      }
      setItems(read);
      void subscriptions
        .markNotificationRead(envelope.notificationId)
        .catch(() => undefined);
      router.push(envelope.route as NotificationRoute);
    },
    [router, sessionId, signedIn, store, subscriptions],
  );

  useEffect(() => {
    clearNotificationBinding();
    if (!signedIn) {
      controller.dispose();
      return;
    }
    signingOutRef.current = false;
    let active = true;
    void store
      .reconcile()
      .then((local) => {
        if (active) setItems(local);
        void subscriptions
          .inbox()
          .then((remote) => store.mergeRemote(remote))
          .then((next) => {
            if (active) setItems(next);
          })
          .catch(() => undefined);
      })
      .catch(() => {
        if (active) setItems([]);
      });
    void controller.inspectAndRepair().catch(() => {
      if (active) setState("error");
    });
    const received = Notifications.addNotificationReceivedListener((event) => {
      void receive(event, false);
    });
    const tapped = Notifications.addNotificationResponseReceivedListener(
      (event) => {
        void receive(event.notification, true);
      },
    );
    const rotated = Notifications.addPushTokenListener(() => {
      void controllerRef.current.repairAfterTokenRotation();
    });
    const coldStart = Notifications.getLastNotificationResponse();
    if (coldStart) {
      Notifications.clearLastNotificationResponse();
      const response = coldStart;
      void receive(response.notification, true);
    }
    return () => {
      clearNotificationBinding(sessionId);
      active = false;
      controller.dispose();
      received.remove();
      tapped.remove();
      rotated.remove();
    };
  }, [controller, receive, sessionId, signedIn, store, subscriptions]);

  useEffect(
    () =>
      authManager.registerSessionClearHook(async (session) => {
        signingOutRef.current = true;
        clearNotificationBinding(session?.sessionId);
        await storeRef.current.clear();
        setItems([]);
        if (!session) return;
        await controllerRef.current.unregisterCurrent(session.accessToken);
      }),
    [],
  );

  const enable = useCallback(async () => {
    await controller.enableFromUserGesture();
  }, [controller]);
  const disable = useCallback(async () => {
    await controller.unregisterCurrent().catch(() => setState("error"));
  }, [controller]);
  const markAllRead = useCallback(async () => {
    const readingSession = sessionId;
    const unreadIDs = items.filter((item) => !item.read).map((item) => item.id);
    const next = await store.markAllRead();
    if (!signingOutRef.current && activeSessionRef.current === readingSession) {
      setItems(next);
    }
    void Promise.allSettled(
      unreadIDs.map((id) => subscriptions.markNotificationRead(id)),
    );
  }, [items, sessionId, store, subscriptions]);
  const open = useCallback(
    async (item: NotificationInboxItem) => {
      const envelope = parseNotificationEnvelope({
        schema: "bex.notification.v1",
        notificationId: item.id,
        event: item.event,
        route: item.route,
        subject,
        workspaceId,
        sessionId,
      });
      if (!envelope || signingOutRef.current) return;
      const openingSession = sessionId;
      const next = await store.markRead(item.id);
      if (
        signingOutRef.current ||
        activeSessionRef.current !== openingSession
      ) {
        return;
      }
      setItems(next);
      void subscriptions.markNotificationRead(item.id).catch(() => undefined);
      router.push(envelope.route as NotificationRoute);
    },
    [router, sessionId, store, subject, workspaceId, subscriptions],
  );
  const value = useMemo(
    () => ({
      state,
      items,
      unread: items.filter((item) => !item.read).length,
      enable,
      disable,
      markAllRead,
      open,
    }),
    [disable, enable, items, markAllRead, open, state],
  );
  return (
    <NotificationsContext.Provider value={value}>
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
