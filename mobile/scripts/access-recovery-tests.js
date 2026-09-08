// Mount real providers and Apollo. Replace only native adapters.
const assert = require("node:assert/strict");
const { test } = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const Module = require("node:module");
const ts = require("typescript");
const React = require("react");
const { createRoot } = require("test-renderer");
const { ApolloClient, ApolloLink, InMemoryCache } = require("@apollo/client");
const { ApolloProvider } = require("@apollo/client/react");
const { Observable } = require("rxjs");
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const listeners = new Set();
const announcements = [];
const routes = [];
const storage = new Map();
const networkListeners = new Set();
let connectivity = "online";
let locale = "en";
let translations;
const translate = (key) =>
  translations
    ? (key
        .split(".")
        .reduce((value, part) => value?.[part], translations[locale]) ?? key)
    : key;
const AppState = {
  currentState: "active",
  addEventListener: (_, listener) => {
    listeners.add(listener);
    return { remove: () => listeners.delete(listener) };
  },
};
const adapters = {
  "react-native": {
    Platform: { OS: "ios", select: (values) => values.ios ?? values.default },
    Appearance: { getColorScheme: () => "light" },
    StyleSheet: { create: (styles) => styles, hairlineWidth: 1 },
    View: "View",
    Text: "Text",
    Pressable: "Pressable",
    TextInput: "TextInput",
    ActivityIndicator: "ActivityIndicator",
    Modal: ({ visible, ...props }) =>
      visible ? React.createElement("Modal", props) : null,
    FlatList: ({ data, renderItem, ...props }) =>
      React.createElement(
        "FlatList",
        props,
        data.map((item, index) =>
          React.createElement(
            React.Fragment,
            { key: item.id ?? index },
            renderItem({ item, index }),
          ),
        ),
      ),
    AppState,
    AccessibilityInfo: {
      announceForAccessibility: (text) => announcements.push(text),
    },
  },
  "expo-router": { router: { replace: (route) => routes.push(route) } },
  "@react-native-async-storage/async-storage": {
    getItem: async (key) => storage.get(key) ?? null,
    setItem: async (key, value) => {
      storage.set(key, value);
    },
    removeItem: async (key) => {
      storage.delete(key);
    },
  },
  "@/features/auth/auth-provider": {
    useAuth: () => ({
      state: { status: "signedIn", session: { sessionId: "test-session" } },
    }),
  },
  "@/common/hooks/use-translations": {
    useTranslations: () => ({ t: translate }),
  },
  "@/common/apollo/network-state": {
    useNetworkState: () =>
      React.useSyncExternalStore(
        (listener) => {
          networkListeners.add(listener);
          return () => networkListeners.delete(listener);
        },
        () => connectivity,
      ),
  },
};
const originalLoad = Module._load;
Module._load = function (request, parent, isMain) {
  if (Object.hasOwn(adapters, request)) return adapters[request];
  if (request.startsWith("@/")) request = path.resolve("src", request.slice(2));
  return originalLoad.call(this, request, parent, isMain);
};
for (const extension of [".ts", ".tsx"]) {
  require.extensions[extension] = (module, filename) => {
    const { outputText } = ts.transpileModule(
      fs.readFileSync(filename, "utf8"),
      {
        fileName: filename,
        compilerOptions: {
          module: ts.ModuleKind.CommonJS,
          jsx: ts.JsxEmit.ReactJSX,
          esModuleInterop: true,
          target: ts.ScriptTarget.ES2022,
        },
      },
    );
    module._compile(outputText, filename);
  };
}
const {
  WorkspaceProvider,
  useWorkspace,
} = require("../src/features/workspaces/workspace-provider");
const {
  CapabilitiesProvider,
  useCapabilities,
} = require("../src/features/capabilities/capabilities-provider");
const { createBoundaryLink } = require("../src/common/apollo/boundary-link");
const { dataBoundary } = require("../src/common/apollo/data-boundary");
const { createAccessLink } = require("../src/common/apollo/access-link");
const { MobileResourceStatusDocument } = require("../src/generated-graphql");
translations = {
  en: require("../src/translations/en").en,
  zh: require("../src/translations/zh").zh,
};
const {
  SafeActionPanel,
} = require("../src/components/safe-action/safe-action-panel");
const { LogSession } = require("../src/features/logs/log-session");
const { LogViewer } = require("../src/features/logs/log-viewer");
const workspaceData = (ids = ["tea-a"]) => ({
  workspaces: ids.map((id) => ({
    id,
    name: id,
    plan: "free",
    role: "Admin",
    createdAt: "2026-09-07T00:00:00Z",
  })),
});
const capabilityData = (outcome = "allowed") => ({
  viewerCapabilities: {
    fresh: true,
    grants: ["can_view", "can_view_logs", "can_operate", "can_create"].map(
      (action) => ({ action, outcome, reason: null }),
    ),
  },
});
async function appState(next) {
  await React.act(async () => {
    AppState.currentState = next;
    for (const listener of listeners) listener(next);
  });
}
test("mounted recovery serializes membership and distinguishes outage from denial", async () => {
  const f = await fixture();
  const { client, requests, reply } = f;
  try {
    assert.equal(f.observed.access.allows("can_view"), false);
    await assert.rejects(
      client.query({
        query: MobileResourceStatusDocument,
        variables: { ownerId: "tea-a" },
        fetchPolicy: "no-cache",
      }),
      /not currently verified/,
    );
    await reply("MobileWorkspaces", workspaceData());
    assert.equal(f.observed.access.allows("can_view"), false);
    const initial = await reply("MobileViewerCapabilities", capabilityData());
    assert.equal(initial.operation.variables.fresh, true);
    assert.equal(f.observed.access.allows("can_operate"), true);
    const realNow = Date.now;
    try {
      const expiredAt = realNow() + 30_000;
      Date.now = () => expiredAt;
      // Dispatch checks time even before React's expiry timer renders.
      assert.equal(f.observed.access.allows("can_view"), false);
      assert.equal(f.observed.access.shows("can_operate"), true);
      await assert.rejects(
        client.query({
          query: MobileResourceStatusDocument,
          variables: { ownerId: "tea-a" },
          fetchPolicy: "no-cache",
        }),
        /not currently verified/,
      );
    } finally {
      Date.now = realNow;
    }
    await appState("background");
    assert.equal(f.observed.access.allows("can_operate"), false);
    assert.equal(f.observed.access.shows("can_operate"), true);
    await appState("active");
    assert.equal(f.observed.access.allows("can_operate"), false);
    assert.deepEqual(
      requests.map(({ operation }) => operation.operationName),
      ["MobileWorkspaces"],
    );
    await assert.rejects(
      client.query({
        query: MobileResourceStatusDocument,
        variables: { ownerId: "tea-a" },
        fetchPolicy: "no-cache",
      }),
      /not currently verified/,
    );
    await reply("MobileWorkspaces", workspaceData());
    const foreground = requests.shift();
    assert.equal(
      foreground.operation.operationName,
      "MobileViewerCapabilities",
    );
    assert.equal(foreground.operation.variables.fresh, true);
    await React.act(async () =>
      foreground.observer.error(new Error("checker offline")),
    );
    assert.equal(f.observed.access.allows("can_operate"), false);
    assert.equal(f.observed.access.shows("can_operate"), true);
    assert.equal(f.observed.access.state.status, "unavailable");
    assert.deepEqual(announcements, []);
    await appState("background");
    await appState("active");
    await reply("MobileWorkspaces", workspaceData());
    await reply("MobileViewerCapabilities", capabilityData("denied"));
    assert.equal(f.observed.access.denied("can_operate"), true);
    assert.equal(f.observed.access.shows("can_operate"), false);
    assert.deepEqual(announcements, [translate("access.changed")]);
    assert.deepEqual(routes, ["/"]);
    assert.ok(dataBoundary.getGeneration() > 0);
    await appState("background");
    await appState("active");
    await reply("MobileWorkspaces", workspaceData([]));
    assert.equal(f.observed.workspace.selected, null);
    assert.equal(f.observed.workspace.status, "empty");
    assert.equal(f.observed.access.allows("can_view"), false);
    assert.equal(storage.has("bex.mobile.workspace.test-session"), false);
    assert.equal(requests.length, 0);
  } finally {
    await f.close();
  }
});

async function fixture(content) {
  storage.clear();
  announcements.length = 0;
  routes.length = 0;
  connectivity = "online";
  AppState.currentState = "active";
  locale = "en";
  await dataBoundary.reset(null);
  const requests = [];
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: ApolloLink.from([
      createBoundaryLink(),
      createAccessLink(),
      new ApolloLink(
        (operation) =>
          new Observable((observer) => {
            requests.push({ operation, observer });
          }),
      ),
    ]),
  });
  let observed;
  let children = content;
  function Probe() {
    observed = { access: useCapabilities(), workspace: useWorkspace() };
    return children?.(observed) ?? null;
  }
  const root = createRoot();
  const render = () =>
    root.render(
      React.createElement(
        ApolloProvider,
        { client },
        React.createElement(
          WorkspaceProvider,
          null,
          React.createElement(
            CapabilitiesProvider,
            null,
            React.createElement(Probe),
          ),
        ),
      ),
    );
  await React.act(async () => render());
  return {
    root,
    client,
    requests,
    get observed() {
      return observed;
    },
    update: async (next) => {
      children = next;
      await React.act(async () => render());
    },
    reply: async (name, data) => {
      const request = requests.shift();
      assert.equal(request?.operation.operationName, name);
      await React.act(async () => {
        request.observer.next({ data });
        request.observer.complete();
      });
      return request;
    },
    close: async () => {
      await React.act(async () => root.unmount());
      client.stop();
    },
  };
}
const treeText = (root) => {
  const visit = (node) =>
    typeof node === "string"
      ? node
      : (node.children ?? []).map(visit).join(" ");
  return visit(root.container);
};
const button = (root, label) =>
  root.container.queryAll(
    (node) =>
      node.type === "Pressable" && node.props.accessibilityLabel === label,
  )[0];
async function connection(next, state = AppState.currentState) {
  await React.act(async () => {
    connectivity = next;
    AppState.currentState = state;
    for (const listener of networkListeners) listener();
    for (const listener of listeners) listener(state);
  });
}
const restartOption = (run, id = "srv-a") => ({
  key: "restart",
  definition: { id: "restart-service", targetKind: "service" },
  target: { id, kind: "service", label: "Private service" },
  label: "Restart",
  run,
});

test("mounted logs and confirmation stop at recovery, reject late stream data, and never replay", async () => {
  let sends = 0;
  const streams = [];
  const session = new LogSession({
    history: async () => ({
      logs: [
        {
          id: "line-a",
          message: "private log contents",
          timestamp: "2026-09-07T12:00:00Z",
          labels: [],
        },
      ],
      hasMore: false,
    }),
    subscribe: async (_, callbacks, signal) => {
      const stream = { callbacks, signal, closed: false };
      streams.push(stream);
      return {
        close: () => {
          stream.closed = true;
        },
      };
    },
  });
  const options = [
    restartOption(async () => {
      sends++;
      return { status: "success" };
    }),
  ];
  const f = await fixture(({ access }) =>
    React.createElement(
      React.Fragment,
      null,
      access.allows("can_view_logs")
        ? React.createElement(LogViewer, { resource: "srv-a", session })
        : null,
      React.createElement(SafeActionPanel, { options }),
    ),
  );
  try {
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.ok(treeText(f.root).includes("private log contents"));
    const oldStream = streams[0];
    await React.act(async () => button(f.root, "Restart").props.onPress());
    const oldConfirm = button(f.root, translate("safeActions.confirm")).props
      .onPress;
    await connection("offline", "background");
    assert.equal(oldStream.closed, true);
    assert.equal(oldStream.signal.aborted, true);
    assert.deepEqual(session.snapshot().lines, []);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      0,
    );
    assert.ok(treeText(f.root).includes(translate("access.offline")));
    await React.act(async () => oldConfirm());
    assert.equal(sends, 0);
    await connection("online", "active");
    assert.deepEqual(
      f.requests.map((item) => item.operation.operationName),
      ["MobileWorkspaces"],
    );
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(sends, 0);
    assert.ok(treeText(f.root).includes("private log contents"));
    await React.act(async () => {
      void f.observed.access.retry();
    });
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData("unavailable"));
    assert.equal(f.observed.access.shows("can_operate"), true);
    assert.equal(f.observed.access.allows("can_operate"), false);
    assert.deepEqual(announcements, []);
    assert.ok(treeText(f.root).includes(translate("access.unavailable")));
    await React.act(async () => {
      void f.observed.access.retry();
    });
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData("denied"));
    assert.deepEqual(announcements, [translate("access.changed")]);
    await React.act(async () =>
      oldStream.callbacks.onLine({
        id: "late",
        message: "late private data",
        timestamp: "2026-09-07T12:00:01Z",
        labels: [],
      }),
    );
    assert.deepEqual(session.snapshot().lines, []);
    assert.ok(!treeText(f.root).includes("private log contents"));
    assert.ok(!treeText(f.root).includes("late private data"));
    assert.deepEqual(routes, ["/"]);
    assert.equal(sends, 0);
  } finally {
    await f.close();
  }
});

test("confirmation checks the latest target, submits once, and preserves ambiguous feedback", async () => {
  let sends = 0;
  let finish;
  const run = () => {
    sends++;
    return new Promise((resolve) => {
      finish = resolve;
    });
  };
  let options = [restartOption(run)];
  const content = () => React.createElement(SafeActionPanel, { options });
  const f = await fixture(content);
  try {
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    await React.act(async () => button(f.root, "Restart").props.onPress());
    const obsolete = button(f.root, translate("safeActions.confirm")).props
      .onPress;
    options = [restartOption(run, "srv-b")];
    await f.update(content);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      0,
    );
    await React.act(async () => obsolete());
    assert.equal(sends, 0);
    await React.act(async () => button(f.root, "Restart").props.onPress());
    const confirm = button(f.root, translate("safeActions.confirm")).props
      .onPress;
    await React.act(async () => {
      confirm();
      confirm();
    });
    assert.equal(sends, 1);
    await React.act(async () => finish({ status: "accepted_unverified" }));
    assert.ok(
      treeText(f.root).includes(
        translate("safeActions.feedback.acceptedUnverified"),
      ),
    );
    locale = "zh";
    await connection("offline");
    assert.ok(treeText(f.root).includes(translations.zh.access.offline));
    await connection("online");
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(sends, 1);
  } finally {
    await f.close();
  }
});

test("creation controls explain a denied operational grant even when creation is allowed", async () => {
  let sends = 0;
  const option = {
    ...restartOption(async () => {
      sends++;
    }),
    definition: { id: "rollback-service", targetKind: "service" },
    label: "Rollback",
  };
  const f = await fixture(() =>
    React.createElement(SafeActionPanel, { options: [option] }),
  );
  try {
    await f.reply("MobileWorkspaces", workspaceData());
    const data = capabilityData();
    data.viewerCapabilities.grants.find(
      (grant) => grant.action === "can_operate",
    ).outcome = "denied";
    await f.reply("MobileViewerCapabilities", data);
    assert.ok(treeText(f.root).includes(translate("access.cannotOpen")));
    assert.equal(button(f.root, "Rollback").props.disabled, true);
    await React.act(async () => button(f.root, "Rollback").props.onPress());
    assert.equal(sends, 0);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      0,
    );
  } finally {
    await f.close();
  }
});

test("workspace switch and identity reset invalidate a mounted confirmation callback", async () => {
  for (const boundary of ["workspace", "identity"]) {
    let sends = 0;
    const f = await fixture(() =>
      React.createElement(SafeActionPanel, {
        options: [
          restartOption(async () => {
            sends++;
            return { status: "success" };
          }),
        ],
      }),
    );
    try {
      await f.reply("MobileWorkspaces", workspaceData(["tea-a", "tea-b"]));
      await f.reply("MobileViewerCapabilities", capabilityData());
      await React.act(async () => button(f.root, "Restart").props.onPress());
      const obsolete = button(f.root, translate("safeActions.confirm")).props
        .onPress;
      if (boundary === "workspace") {
        await React.act(async () =>
          f.observed.workspace.switchWorkspace("tea-b"),
        );
        await f.reply("MobileViewerCapabilities", capabilityData());
      } else {
        await React.act(async () => dataBoundary.reset(null));
      }
      assert.equal(
        f.root.container.queryAll((node) => node.type === "Modal").length,
        0,
      );
      await React.act(async () => obsolete());
      assert.equal(sends, 0, boundary);
    } finally {
      await f.close();
    }
  }
});

test("foreground retries failed startup membership before any protected request", async () => {
  const f = await fixture();
  try {
    const startup = f.requests.shift();
    assert.equal(startup.operation.operationName, "MobileWorkspaces");
    await React.act(async () =>
      startup.observer.error(new Error("offline during startup")),
    );
    assert.equal(f.observed.workspace.status, "error");
    assert.equal(f.observed.access.allows("can_view"), false);
    await appState("background");
    await appState("active");
    await f.reply("MobileWorkspaces", workspaceData());
    assert.equal(f.requests[0].operation.variables.fresh, true);
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(f.observed.workspace.status, "ready");
    assert.equal(f.observed.access.allows("can_view"), true);
  } finally {
    await f.close();
  }
});

test("identity reset aborts an actual Apollo request and rejects its late protected payload", async () => {
  const f = await fixture();
  try {
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    const result = f.client
      .query({
        query: MobileResourceStatusDocument,
        variables: { ownerId: "tea-a" },
        fetchPolicy: "network-only",
      })
      .then(
        (value) => value.data,
        () => null,
      );
    const request = f.requests.shift();
    assert.equal(request.operation.operationName, "MobileResourceStatus");
    const signal = request.operation.getContext().fetchOptions.signal;
    assert.equal(signal.aborted, false);
    await React.act(async () => dataBoundary.reset(null));
    assert.equal(signal.aborted, true);
    await React.act(async () => {
      request.observer.next({
        data: {
          projects: [],
          services: [],
          databases: [],
          keyValues: [],
        },
      });
      request.observer.complete();
    });
    assert.equal(await result, null);
    assert.equal(
      f.client.cache.readQuery({
        query: MobileResourceStatusDocument,
        variables: { ownerId: "tea-a" },
      }),
      null,
    );
    assert.equal(f.observed.access.allows("can_view"), false);
  } finally {
    await f.close();
  }
});

test("a newer boundary supersedes an access upgrade waiting for reset handlers", async () => {
  const f = await fixture();
  const finishes = [];
  let removeHandler = () => {};
  try {
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData("denied"));
    removeHandler = dataBoundary.registerResetHandler(
      () => new Promise((resolve) => finishes.push(resolve)),
    );
    await appState("background");
    await appState("active");
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(finishes.length, 1);
    let newerReset;
    await React.act(async () => {
      newerReset = dataBoundary.reset("tea-a");
    });
    assert.equal(finishes.length, 2);
    await React.act(async () => {
      finishes.forEach((finish) => finish());
      await newerReset;
    });
    assert.equal(f.observed.access.allows("can_operate"), false);
    assert.deepEqual(routes, []);
    assert.deepEqual(announcements, []);
  } finally {
    removeHandler();
    finishes.forEach((finish) => finish());
    await f.close();
  }
});

test("membership removal requires explicit selection from verified remaining workspaces", async () => {
  const f = await fixture();
  try {
    await f.reply("MobileWorkspaces", workspaceData(["tea-a", "tea-b"]));
    await f.reply("MobileViewerCapabilities", capabilityData());
    await appState("background");
    await appState("active");
    await f.reply("MobileWorkspaces", workspaceData(["tea-b"]));
    assert.equal(f.observed.workspace.status, "choose");
    assert.equal(f.observed.workspace.selected, null);
    assert.deepEqual(
      f.observed.workspace.workspaces.map((workspace) => workspace.id),
      ["tea-b"],
    );
    assert.equal(f.requests.length, 0);
    await React.act(async () => f.observed.workspace.switchWorkspace("tea-b"));
    assert.equal(f.observed.access.allows("can_view"), false);
    const request = await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(request.operation.variables.ownerId, "tea-b");
    assert.equal(f.observed.workspace.selected.id, "tea-b");
    assert.equal(f.observed.access.allows("can_view"), true);
  } finally {
    await f.close();
  }
});

test("equal-grant polling preserves mounted logs and confirmation until actual expiry", async (t) => {
  t.mock.timers.enable({
    apis: ["Date", "setTimeout", "setInterval"],
    now: 1_800_000_000_000,
  });
  let histories = 0;
  let closes = 0;
  const session = new LogSession({
    history: async () => {
      histories++;
      return { logs: [], hasMore: false };
    },
    subscribe: async () => ({
      close: () => {
        closes++;
      },
    }),
  });
  const options = [restartOption(async () => ({ status: "success" }))];
  const f = await fixture(({ access }) =>
    React.createElement(
      React.Fragment,
      null,
      access.allows("can_view_logs")
        ? React.createElement(LogViewer, { resource: "srv-a", session })
        : null,
      React.createElement(SafeActionPanel, { options }),
    ),
  );
  try {
    await f.reply("MobileWorkspaces", workspaceData());
    await f.reply("MobileViewerCapabilities", capabilityData());
    await React.act(async () => button(f.root, "Restart").props.onPress());
    assert.equal(histories, 1);
    await React.act(async () => t.mock.timers.tick(25_000));
    assert.equal(f.observed.access.allows("can_operate"), true);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      1,
    );
    assert.equal(closes, 0);
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(histories, 1);
    assert.equal(closes, 0);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      1,
    );
    await React.act(async () => t.mock.timers.tick(25_000));
    assert.equal(f.requests.length, 1);
    await React.act(async () => t.mock.timers.tick(5_000));
    assert.equal(f.observed.access.allows("can_view"), false);
    assert.equal(f.observed.access.shows("can_operate"), true);
    assert.equal(closes, 1);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      0,
    );
    await f.reply("MobileViewerCapabilities", capabilityData());
    assert.equal(histories, 2);
    assert.equal(
      f.root.container.queryAll((node) => node.type === "Modal").length,
      0,
    );
  } finally {
    await f.close();
    t.mock.timers.reset();
  }
});
