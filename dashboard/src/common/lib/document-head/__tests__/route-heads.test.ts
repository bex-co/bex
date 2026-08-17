import { afterEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n/init";
import type { DashboardHead, RouteResource } from "@/common/lib/document-head";
import { Route as ConsentRoute } from "@/routes/auth.consent";
import { Route as DeviceSuccessRoute } from "@/routes/auth.device.success";
import { Route as DeviceRoute } from "@/routes/auth.device";
import { Route as ForgotPasswordRoute } from "@/routes/auth.forgot-password";
import { Route as LoginRoute } from "@/routes/auth.login";
import { Route as LogoutRoute } from "@/routes/auth.logout";
import { Route as ResetPasswordRoute } from "@/routes/auth.reset-password";
import { Route as SignUpRoute } from "@/routes/auth.sign-up";
import { Route as VerificationRoute } from "@/routes/auth.verification";
import { Route as BlueprintsRoute } from "@/routes/blueprints";
import { Route as EnvGroupsRoute } from "@/routes/env-groups";
import { Route as IndexRoute } from "@/routes/index";
import { Route as NewKeyValueRoute } from "@/routes/keyvalue.new";
import { Route as NewWorkspaceRoute } from "@/routes/new.workspace";
import { Route as NotificationsRoute } from "@/routes/notifications";
import { Route as ProjectRoute } from "@/routes/project.$projectId";
import { Route as ProjectSettingsRoute } from "@/routes/project.$projectId.settings";
import { Route as ServiceRoute } from "@/routes/services.$serviceId";
import { Route as NewServiceRoute } from "@/routes/services.new";
import { Route as SettingsRoute } from "@/routes/settings";
import { Route as BillingRoute } from "@/routes/billing";
import { Route as WebhookRoute } from "@/routes/webhook.$webhookId";
import { Route as WebhooksRoute } from "@/routes/webhooks";
import { Route as NewWebhookRoute } from "@/routes/webhooks_.new";
import { Route as WorkspaceSettingsRoute } from "@/routes/workspace.settings";
import { Route as DatabaseRoute } from "@/routes/databases.$databaseId";
import { Route as KeyValueRoute } from "@/routes/keyvalue.$keyValueId";
import { Route as EnvGroupRoute } from "@/routes/env-groups_.$groupId";
import { Route as BlueprintRoute } from "@/routes/blueprints.$blueprintId";

type HeadOwner = {
  options: {
    head?: unknown;
  };
};

type LoaderOwner = HeadOwner & {
  options: {
    loader?: unknown;
  };
};

function routeTitle(
  route: HeadOwner,
  loaderData?: unknown,
  match: Record<string, unknown> = { search: {} },
): string {
  const buildHead = route.options.head as (context: {
    loaderData?: unknown;
    match: Record<string, unknown>;
  }) => DashboardHead;
  const head = buildHead({ loaderData, match });
  const title = head.meta.find(
    (entry): entry is { title: string } => "title" in entry,
  );
  if (!title) throw new Error("Route head did not return a title");
  return title.title;
}

function ready<T>(resource: T): RouteResource<T> {
  return { state: "ready", resource };
}

const staticRouteCases: Array<
  [
    label: string,
    route: HeadOwner,
    match: Record<string, unknown>,
    english: string,
    chinese: string,
  ]
> = [
  [
    "OAuth consent",
    ConsentRoute,
    { search: {} },
    "Authorize access",
    "授权访问",
  ],
  [
    "device success",
    DeviceSuccessRoute,
    { search: {} },
    "Render CLI connected",
    "Render CLI 已连接",
  ],
  [
    "device verification",
    DeviceRoute,
    { search: {} },
    "Connect Render CLI",
    "连接 Render CLI",
  ],
  [
    "forgot password",
    ForgotPasswordRoute,
    { search: {} },
    "Reset your password",
    "重置您的密码",
  ],
  [
    "login",
    LoginRoute,
    { search: {} },
    "Sign in to your account",
    "登录您的账户",
  ],
  ["logout", LogoutRoute, { search: {} }, "Sign out", "退出登录"],
  [
    "reset password",
    ResetPasswordRoute,
    { search: {} },
    "Reset your password",
    "重置您的密码",
  ],
  [
    "sign up",
    SignUpRoute,
    { search: {} },
    "Create your account",
    "创建您的账户",
  ],
  [
    "email verification",
    VerificationRoute,
    { search: {} },
    "Verify your email",
    "验证您的邮箱",
  ],
  ["Blueprints", BlueprintsRoute, { search: {} }, "Blueprints", "蓝图"],
  [
    "Environment Groups",
    EnvGroupsRoute,
    { search: {} },
    "Environment Groups",
    "环境变量组",
  ],
  ["Overview", IndexRoute, { search: {} }, "Overview", "概览"],
  [
    "new project",
    IndexRoute,
    { search: { new: "project" } },
    "New Project",
    "新建项目",
  ],
  [
    "new Postgres",
    IndexRoute,
    { search: { new: "database" } },
    "New Postgres",
    "新建 Postgres 数据库",
  ],
  [
    "new Key Value",
    NewKeyValueRoute,
    { search: {} },
    "New Key Value",
    "新建键值存储",
  ],
  [
    "new workspace",
    NewWorkspaceRoute,
    { search: {} },
    "New Workspace",
    "新建工作区",
  ],
  [
    "notifications",
    NotificationsRoute,
    { search: {} },
    "Notifications",
    "通知",
  ],
  ["new service", NewServiceRoute, { search: {} }, "New Service", "新建服务"],
  ["account settings", SettingsRoute, { search: {} }, "Settings", "设置"],
  ["billing", BillingRoute, { search: {} }, "Billing", "账单"],
  ["webhooks", WebhooksRoute, { search: {} }, "Webhooks", "Webhooks"],
  [
    "new webhook",
    NewWebhookRoute,
    { search: {} },
    "Create a new webhook",
    "创建新 Webhook",
  ],
  [
    "workspace settings",
    WorkspaceSettingsRoute,
    { search: {} },
    "Workspace Settings",
    "工作区设置",
  ],
];

async function runRouteLoader(
  route: LoaderOwner,
  params: Record<string, string>,
  data: Record<string, unknown>,
  workspaceId: string | null = "tea-selfhost",
  error?: Error,
) {
  const query = vi.fn(async () => ({ data, error }));
  const loader = route.options.loader as (context: {
    context: { client: { query: typeof query }; workspaceId: string | null };
    params: Record<string, string>;
  }) => Promise<unknown>;
  const result = await loader({
    context: { client: { query }, workspaceId },
    params,
  });
  return { query, result };
}

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("production route heads", () => {
  it.each(staticRouteCases)(
    "uses the shared localized contract for $label",
    async (_label, route, match, english, chinese) => {
      expect(routeTitle(route, undefined, match)).toBe(
        `${english} ・ bex Dashboard`,
      );
      await i18n.changeLanguage("zh");
      expect(routeTitle(route, undefined, match)).toBe(
        `${chinese} ・ bex Dashboard`,
      );
    },
  );

  it("uses project names and Render's name-first settings hierarchy", async () => {
    const result = ready({ id: "prj-private", name: "storefront" });

    expect(routeTitle(ProjectRoute, result)).toBe(
      "storefront ・ bex Dashboard",
    );
    expect(routeTitle(ProjectSettingsRoute, result)).toBe(
      "storefront / Settings ・ bex Dashboard",
    );

    await i18n.changeLanguage("zh");
    expect(routeTitle(ProjectSettingsRoute, result)).toBe(
      "storefront / 设置 ・ bex Dashboard",
    );
  });

  it.each([
    ["web_service", "Web Service"],
    ["private_service", "Private Service"],
    ["background_worker", "Background Worker"],
    ["cron_job", "Cron Job"],
    ["static_site", "Static Site"],
  ])("titles %s services from the SSR resource result", (type, label) => {
    const title = routeTitle(
      ServiceRoute,
      ready({
        id: "srv-private",
        name: "api",
        displayName: "friendly-api",
        type,
      }),
    );

    expect(title).toBe(`friendly-api ・ ${label} ・ bex Dashboard`);
    expect(title).not.toContain("srv-private");
  });

  it("translates the service type without changing its human name", async () => {
    await i18n.changeLanguage("zh");
    expect(
      routeTitle(
        ServiceRoute,
        ready({ name: "nightly", displayName: null, type: "cron_job" }),
      ),
    ).toBe("nightly ・ 定时任务 ・ bex Dashboard");
  });

  it("settles every other named resource on its human name and type", () => {
    const cases: Array<[HeadOwner, string, string, string]> = [
      [DatabaseRoute, "dpg-private", "orders", "Database"],
      [KeyValueRoute, "red-private", "cache", "Key Value"],
      [EnvGroupRoute, "evg-private", "production", "Environment Group"],
      [BlueprintRoute, "blp-private", "platform", "Blueprint"],
      [WebhookRoute, "whk-private", "deploy-events", "Webhook"],
    ];

    for (const [route, id, name, type] of cases) {
      const title = routeTitle(route, ready({ id, name }));
      expect(title).toBe(`${name} ・ ${type} ・ bex Dashboard`);
      expect(title).not.toContain(id);
    }

    expect(
      routeTitle(
        WebhookRoute,
        ready({
          id: "whk-private",
          name: "",
          url: "https://hooks.example.test/deploys",
        }),
      ),
    ).toBe("https://hooks.example.test/deploys ・ Webhook ・ bex Dashboard");
  });

  it("translates datastore and named-resource types", async () => {
    await i18n.changeLanguage("zh");
    expect(routeTitle(DatabaseRoute, ready({ name: "orders" }))).toBe(
      "orders ・ 数据库 ・ bex Dashboard",
    );
    expect(routeTitle(KeyValueRoute, ready({ name: "cache" }))).toBe(
      "cache ・ 键值存储 ・ bex Dashboard",
    );
    expect(routeTitle(EnvGroupRoute, ready({ name: "production" }))).toBe(
      "production ・ 环境变量组 ・ bex Dashboard",
    );
    expect(routeTitle(BlueprintRoute, ready({ name: "platform" }))).toBe(
      "platform ・ 蓝图 ・ bex Dashboard",
    );
    expect(routeTitle(WebhookRoute, ready({ name: "deploy-events" }))).toBe(
      "deploy-events ・ Webhook ・ bex Dashboard",
    );
  });

  it("replaces a previous private title in loading, error, and not-found states", () => {
    document.title = "secret-project ・ bex Dashboard";

    expect(routeTitle(ServiceRoute)).toBe("Loading… ・ bex Dashboard");
    expect(routeTitle(ServiceRoute, { state: "error" })).toBe(
      "Something went wrong ・ bex Dashboard",
    );
    expect(routeTitle(ServiceRoute, { state: "not-found" })).toBe(
      "Page not found ・ bex Dashboard",
    );
  });

  it("loads each dynamic head resource exactly once through its authenticated route seam", async () => {
    const cases: Array<
      [LoaderOwner, Record<string, string>, Record<string, unknown>, string]
    > = [
      [
        ProjectRoute,
        { projectId: "prj-private" },
        { project: { id: "prj-private", name: "storefront" } },
        "storefront",
      ],
      [
        ServiceRoute,
        { serviceId: "srv-private" },
        { server: { id: "srv-private", name: "api", type: "web_service" } },
        "api",
      ],
      [
        DatabaseRoute,
        { databaseId: "dpg-private" },
        { database: { id: "dpg-private", name: "orders" } },
        "orders",
      ],
      [
        KeyValueRoute,
        { keyValueId: "red-private" },
        { keyValue: { id: "red-private", name: "cache" } },
        "cache",
      ],
      [
        EnvGroupRoute,
        { groupId: "evg-private" },
        { envGroup: { id: "evg-private", name: "production" } },
        "production",
      ],
      [
        BlueprintRoute,
        { blueprintId: "blp-private" },
        { blueprint: { id: "blp-private", name: "platform" } },
        "platform",
      ],
      [
        WebhookRoute,
        { webhookId: "whk-private" },
        {
          webhookEndpoint: {
            id: "whk-private",
            name: "deploy-events",
            url: "https://hooks.example.test/deploys",
          },
        },
        "deploy-events",
      ],
    ];

    for (const [route, params, data, expectedName] of cases) {
      const { query, result } = await runRouteLoader(route, params, data);
      expect(query).toHaveBeenCalledOnce();
      expect(query.mock.calls[0]?.[0]).toMatchObject({
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      expect(result).toMatchObject({
        state: "ready",
        resource: { name: expectedName },
      });
    }
  });

  it.each([
    ["project", ProjectRoute, { projectId: "prj-missing" }, { project: {} }],
    ["service", ServiceRoute, { serviceId: "srv-missing" }, { server: {} }],
    [
      "database",
      DatabaseRoute,
      { databaseId: "dpg-missing" },
      { database: {} },
    ],
    [
      "Key Value",
      KeyValueRoute,
      { keyValueId: "red-missing" },
      { keyValue: {} },
    ],
    [
      "Environment Group",
      EnvGroupRoute,
      { groupId: "evg-missing" },
      { envGroup: {} },
    ],
    [
      "Blueprint",
      BlueprintRoute,
      { blueprintId: "blp-missing" },
      { blueprint: {} },
    ],
    [
      "Webhook",
      WebhookRoute,
      { webhookId: "whk-missing" },
      { webhookEndpoint: {} },
    ],
  ] as Array<
    [string, LoaderOwner, Record<string, string>, Record<string, unknown>]
  >)(
    "does not treat an empty %s adapter object as a ready named resource",
    async (_label, route, params, data) => {
      const { result } = await runRouteLoader(route, params, data);
      expect(result).toEqual({ state: "not-found" });
    },
  );
});
