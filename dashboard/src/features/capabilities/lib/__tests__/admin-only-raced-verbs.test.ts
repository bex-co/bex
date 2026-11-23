import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { ADMIN_ONLY_REASON } from "@/test/mocks/admin-only";
import type { DatabaseView } from "@/features/databases/types";
import type { ServiceView } from "@/features/services/types";

// Every useMutation rejects with what the next test arms.
let refusal: Error = new Error("unarmed");
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [
    vi.fn(async () => {
      throw refusal;
    }),
    {},
  ],
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: (m: string) => toastError(m) },
}));

import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import { useDeleteDatabase } from "@/features/databases/hooks/use-delete-database";
import { useDeleteKeyValue } from "@/features/keyvalue/hooks/use-delete-key-value";
import { useKeyValueLifecycle } from "@/features/keyvalue/hooks/use-key-value-lifecycle";
import { useDeleteService } from "@/features/services/hooks/use-delete-service";
import { useMaintenanceMode } from "@/features/services/hooks/use-maintenance-mode";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";

const service = { id: "srv-web", name: "web" } as ServiceView;
const database = { id: "dpg-orders", name: "orders" } as DatabaseView;

function serviceLifecycle() {
  return renderHook(() =>
    useServiceLifecycle({ refetch: async () => [], pollIntervalMs: 0 }),
  ).result;
}

function databaseLifecycle() {
  return renderHook(() => useDatabaseLifecycle({ refetch: () => undefined }))
    .result;
}

function keyValueLifecycle() {
  return renderHook(() => useKeyValueLifecycle()).result;
}

// Each verb a protected environment keeps to Admins, as the dashboard fires
// it: each entry renders its hook and answers the call that fires the verb.
const VERBS: [string, () => () => Promise<unknown>][] = [
  [
    "a service's suspend",
    () => {
      const hook = serviceLifecycle();
      return () => hook.current.run("suspend", service);
    },
  ],
  [
    "a service's resume",
    () => {
      const hook = serviceLifecycle();
      return () => hook.current.run("resume", service);
    },
  ],
  [
    "a service's delete",
    () => {
      const hook = renderHook(() => useDeleteService()).result;
      return () => hook.current.remove(service.id, service.name);
    },
  ],
  [
    "turning a service's maintenance mode on",
    () => {
      const hook = renderHook(() => useMaintenanceMode()).result;
      return () => hook.current.setMaintenanceMode(service.id, true, "");
    },
  ],
  [
    "a Postgres's suspend",
    () => {
      const hook = databaseLifecycle();
      return () => hook.current.run("suspend", database);
    },
  ],
  [
    "a Postgres's resume",
    () => {
      const hook = databaseLifecycle();
      return () => hook.current.run("resume", database);
    },
  ],
  [
    "a Postgres's delete",
    () => {
      const hook = renderHook(() => useDeleteDatabase()).result;
      return () => hook.current.remove(database.id, database.name);
    },
  ],
  [
    "a Key Value's suspend",
    () => {
      const hook = keyValueLifecycle();
      return () => hook.current.run("suspend", "red-cache", "cache");
    },
  ],
  [
    "a Key Value's resume",
    () => {
      const hook = keyValueLifecycle();
      return () => hook.current.run("resume", "red-cache", "cache");
    },
  ],
  [
    "a Key Value's delete",
    () => {
      const hook = renderHook(() => useDeleteKeyValue()).result;
      return () => hook.current.remove("red-cache", "cache");
    },
  ],
];

beforeEach(() => {
  toastError.mockReset();
});

// w4/728: bex-api codes a protected environment's refusal of a non-Admin
// PROTECTED_ENVIRONMENT_ADMIN_REQUIRED for suspend, resume, delete and
// maintenance mode too, so a click that raced the disabled control reads the
// reason that control shows, whatever the server's words.
describe("a raced admin-only refusal", () => {
  it.each(VERBS)(
    "of %s reads Render's admin-only reason",
    async (_, render) => {
      refusal = codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED", {
        environmentId: "evm-1",
      });
      const fire = render();
      await act(async () => {
        await fire();
      });
      expect(toastError).toHaveBeenCalledExactlyOnceWith(ADMIN_ONLY_REASON);
    },
  );

  it.each(VERBS)("of %s keeps any other refusal's words", async (_, render) => {
    refusal = codedGraphQLError("FORBIDDEN", {}, "forbidden");
    const fire = render();
    await act(async () => {
      await fire();
    });
    expect(toastError).toHaveBeenCalledOnce();
    expect(toastError).not.toHaveBeenCalledWith(ADMIN_ONLY_REASON);
  });
});
