import type {
  MemberRole,
  PermissionAreaView,
  PermissionLevel,
  PermissionLevelView,
} from "@/features/api-keys/types";

// A slice of bex-api's permission catalog (core.KeyPermissionCatalog) as a
// Developer sees it: Services offers Read, Write and Read & write; Sandbox
// execution only Execute; Audit logs needs an Admin; Git (a bex area) lets a
// Developer grant Read but not Read & write.

function level(
  id: PermissionLevel,
  name: string,
  grantable: boolean,
  requiredRole: MemberRole,
  actions: string[],
): PermissionLevelView {
  return {
    level: id,
    name,
    grantable,
    requiredRole,
    actions: actions.map((action) => ({
      id: action.toLowerCase().replace(/\W+/g, "."),
      name: action,
      bex: false,
    })),
  };
}

export const CATALOG: PermissionAreaView[] = [
  {
    id: "services",
    name: "Services",
    bex: false,
    permissions: [
      {
        id: "services",
        name: "Web and private services, static sites, background workers, and cron jobs",
        levels: [
          level("read", "Read", true, "viewer", [
            "View services and their configuration",
          ]),
          level("read_write", "Read & write", true, "developer", [
            "View services and their configuration",
            "Create, update, and delete services",
          ]),
        ],
      },
      {
        id: "services_env",
        name: "Environment variables and secret files",
        levels: [
          level("read", "Read", true, "developer", [
            "View environment variables and secret files",
          ]),
          level("write", "Write", true, "developer", [
            "Set environment variable and secret file values",
          ]),
          level("read_write", "Read & write", true, "developer", [
            "View environment variables and secret files",
            "Set environment variable and secret file values",
          ]),
        ],
      },
    ],
  },
  {
    id: "sandboxes",
    name: "Sandboxes",
    bex: false,
    permissions: [
      {
        id: "sandbox_execution",
        name: "Sandbox execution and file transfer",
        levels: [
          level("execute", "Execute", true, "developer", [
            "Run commands in sandboxes",
          ]),
        ],
      },
    ],
  },
  {
    id: "audit_logs",
    name: "Audit logs",
    bex: false,
    permissions: [
      {
        id: "audit_logs",
        name: "Read workspace audit logs",
        levels: [
          level("read", "Read", false, "admin", ["Read workspace audit logs"]),
        ],
      },
    ],
  },
  {
    id: "git",
    name: "Git",
    bex: true,
    permissions: [
      {
        id: "git",
        name: "Git connections and repositories",
        levels: [
          level("read", "Read", true, "viewer", [
            "View Git connections and repositories",
          ]),
          level("read_write", "Read & write", false, "admin", [
            "View Git connections and repositories",
            "Connect and disconnect Git providers",
          ]),
        ],
      },
    ],
  },
];

/** What the All preset shows for CATALOG: each permission at its broadest grantable level. */
export const ALL_LEVELS: Record<string, PermissionLevel> = {
  services: "read_write",
  services_env: "read_write",
  sandbox_execution: "execute",
  git: "read",
};

/** What the Read only preset selects for CATALOG. */
export const READ_ONLY_LEVELS: Record<string, PermissionLevel> = {
  services: "read",
  services_env: "read",
  git: "read",
};
