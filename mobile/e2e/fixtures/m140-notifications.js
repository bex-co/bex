// Development runtime only: inject via Hermes CDP, never import from app code.
(async () => {
  const modules = Array.from(__r.getModules());
  const get = (name) =>
    __r(modules.find(([id, m]) => m.verboseName === name)[0]);
  const { ApolloLink } = get("node_modules/@apollo/client/core/index.js");
  const { Observable } = get(
    "node_modules/rxjs/dist/esm5/internal/Observable.js",
  );
  const { apolloClient: client } = get("src/common/apollo/apollo-client.ts");
  const { authManager: auth } = get("src/features/auth/auth-provider.tsx");
  const boundary = get("src/common/apollo/data-boundary.ts");
  const { createBoundaryLink } = get("src/common/apollo/boundary-link.ts");
  const { createAccessLink } = get("src/common/apollo/access-link.ts");
  const rn = get("node_modules/react-native/index.js");
  const router = get("node_modules/expo-router/build/exports.js").router;
  const f = (globalThis.__m140 = {
    mode: "allowed",
    workspaces: ["tea-a", "tea-b"],
    requests: [],
    mutations: [],
    announcements: [],
    originalLink: client.link,
    originalAnnounce: rn.AccessibilityInfo.announceForAccessibility,
    boundary,
    router,
  });
  rn.AccessibilityInfo.announceForAccessibility = (text) => {
    f.announcements.push(text);
    f.originalAnnounce(text);
  };
  const emitter = get(
    "node_modules/expo-notifications/build/NotificationsEmitter.js",
  );
  const asyncStorage = get(
    "node_modules/@react-native-async-storage/async-storage/src/index.ts",
  ).default;
  f.storage = asyncStorage;
  f.emitter = emitter;
  f.received = new Set();
  f.tapped = new Set();
  f.routes = [];
  f.pendingCapabilities = [];
  f.holdCapabilities = true;
  f.originalPush = router.push;
  router.push = (route, ...args) => {
    f.routes.push(route);
    return f.originalPush(route, ...args);
  };
  f.response = (
    workspaceId = "tea-b",
    event = "server_failed",
    route = "/services/srv-native",
    id = "notification-native",
  ) => ({
    actionIdentifier: "expo.modules.notifications.actions.DEFAULT",
    notification: {
      date: Date.now(),
      request: {
        identifier: id,
        trigger: null,
        content: {
          title: "Mobile QA alert",
          body: "Synthetic notification evidence",
          data: {
            schema: "bex.notification.v1",
            notificationId: id,
            event,
            route,
            subject: "synthetic-native-qa",
            sessionId: "m140-native-fixture",
            workspaceId,
          },
        },
      },
    },
  });
  f.initial = f.response();
  f.originalEmitter = {};
  const wrapListener = (name, callbacks) => {
    const original = emitter[name];
    f.originalEmitter[name] = original;
    emitter[name] = (callback) => {
      callbacks.add(callback);
      const subscription = original(callback);
      return {
        remove: () => {
          callbacks.delete(callback);
          subscription.remove();
        },
      };
    };
  };
  wrapListener("addNotificationReceivedListener", f.received);
  wrapListener("addNotificationResponseReceivedListener", f.tapped);
  f.originalEmitter.getLastNotificationResponse =
    emitter.getLastNotificationResponse;
  f.originalEmitter.clearLastNotificationResponse =
    emitter.clearLastNotificationResponse;
  emitter.getLastNotificationResponse = () => f.initial;
  emitter.clearLastNotificationResponse = () => {
    f.initial = null;
  };
  f.remote = [];
  const timestamp = new Date().toISOString();
  const service = {
    id: "srv-native",
    name: "m140-native-fixture",
    displayName: "Access recovery fixture",
    type: "web_service",
    runtime: "docker",
    phase: "Running",
    suspended: "not_suspended",
    replicas: 1,
    revision: 1,
    region: "test",
    latestDeployId: null,
    projectId: null,
    updatedAt: timestamp,
  };
  const agentSession = {
    id: "ags-native",
    repo: "test/mobile-access",
    branch: "main",
    phase: "running",
    status: "running",
    headSha: null,
    prUrl: null,
    prNumber: null,
    failureReason: null,
    turns: 1,
    deliveryMode: "pull_request",
    createdAt: timestamp,
    updatedAt: timestamp,
    canceledAt: null,
  };
  const resolve = (name) => {
    switch (name) {
      case "MobileWorkspaces":
        return {
          workspaces: f.workspaces.map((id) => ({
            id,
            name: id === "tea-b" ? "Mobile QA B" : "Mobile QA A",
            plan: "free",
            role: "Contributor",
            createdAt: timestamp,
          })),
        };
      case "MobileViewerCapabilities":
        return {
          viewerCapabilities: {
            fresh: true,
            grants: [
              "can_view",
              "can_view_logs",
              "can_operate",
              "can_create",
            ].map((action) => ({
              action,
              outcome:
                f.mode === "unavailable"
                  ? "unavailable"
                  : f.mode === "viewer" &&
                      (action === "can_operate" || action === "can_create")
                    ? "denied"
                    : "allowed",
              reason: null,
            })),
          },
        };
      case "MobileResourceStatus":
        return {
          projects: [],
          services: [service],
          databases: [],
          keyValues: [],
        };
      case "MobileUsageGlance":
        return {
          usage: {
            period: "2026-09",
            coverage: {
              state: "complete",
              through: timestamp,
              degradedSources: [],
            },
            services: [],
          },
        };
      case "MobileServiceSupervision":
        return { service };
      case "MobileDeployHistory":
        return { deploys: [] };
      case "MobileServiceEvents":
        return { serviceEvents: [] };
      case "MobileMetricSnapshot":
        return { metricSnapshot: null };
      case "MobileAgentSessions":
        return { agentSessions: [agentSession] };
      case "MobileAgentSession":
        return { agentSession };
      case "MobileAgentSessionCapabilities":
        return {
          agentSessionCapabilities: {
            enabled: true,
            ready: false,
            modelKeyReady: false,
            github: { connected: false, accountLogin: null, installUrl: null },
            agents: [],
          },
        };
      case "MobileAgentRepos":
        return { repos: [] };
      case "MobileNotificationDeviceSubscriptions":
        return {
          pushNotificationsAvailable: false,
          notificationDeviceSubscriptions: [],
        };
      case "MobileNotificationInbox":
        return {
          notificationInbox:
            f.mode === "viewer"
              ? f.remote.filter((x) => !x.deepLink.startsWith("/sessions/"))
              : f.remote,
          unreadPushNotificationCount: f.remote.length,
        };
      case "MobileMarkPushNotificationRead":
        return { markPushNotificationRead: true };
      default:
        throw new Error("Fixture has no response for " + name);
    }
  };
  client.setLink(
    ApolloLink.from([
      createBoundaryLink(),
      createAccessLink(),
      new ApolloLink(
        (operation) =>
          new Observable((observer) => {
            f.requests.push({
              name: operation.operationName,
              at: Date.now(),
              ownerId: operation.variables.ownerId,
              fresh: operation.variables.fresh,
            });
            if (
              operation.operationName === "MobileViewerCapabilities" &&
              f.holdCapabilities
            ) {
              f.pendingCapabilities.push({ observer, operation });
              return;
            }
            if (
              operation.operationType === "mutation" &&
              operation.operationName !== "MobileMarkPushNotificationRead"
            ) {
              f.mutations.push(operation.operationName);
              observer.error(new Error("Fixture forbids resource mutations"));
              return;
            }
            try {
              observer.next({ data: resolve(operation.operationName) });
              observer.complete();
            } catch (e) {
              observer.error(e);
            }
          }),
      ),
    ]),
  );
  f.release = () => {
    f.holdCapabilities = false;
    for (const request of f.pendingCapabilities.splice(0)) {
      request.observer.next({ data: resolve("MobileViewerCapabilities") });
      request.observer.complete();
    }
  };
  f.emit = (response, tapped = false) => {
    for (const callback of tapped ? f.tapped : f.received)
      callback(tapped ? response : response.notification);
  };
  await asyncStorage.setItem(
    "bex.mobile.workspace.m140-native-fixture",
    "tea-b",
  );
  await boundary.resetIdentityBoundary();
  auth.setState({
    status: "signedIn",
    session: {
      version: 1,
      sessionId: "m140-native-fixture",
      subject: "synthetic-native-qa",
      issuer: "https://synthetic.invalid",
      clientId: "synthetic",
      accessToken: "synthetic-not-a-credential",
      refreshToken: "synthetic-not-a-credential",
      expiresAt: Date.now() + 3600000,
      scope: "synthetic",
    },
  });
  return { fixture: true, mode: f.mode };
})();
