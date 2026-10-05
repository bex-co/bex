import { describe, expect, it } from "vitest";

import { isReservedPostgresRole } from "@/features/databases/lib/identifiers";

// Mirrors TestReservedPostgresRole (lego/types/v1alpha1): the names CNPG
// reserves (its webhook refuses the whole Cluster update for them) and those
// PostgreSQL refuses to create, so the form never offers a role bex-api refuses.
describe("isReservedPostgresRole", () => {
  it.each([
    ["postgres", true],
    ["streaming_replica", true],
    ["cnpg_pooler_pgbouncer", true],
    ["cnpg_reader", true],
    ["public", true],
    ["none", true],
    ["pg_monitor", true],
    ["pg_reader", true],
    ["app_user", false],
    ["orders_owner", false],
    ["cnpg", false],
    ["pg", false],
    ["publicist", false],
    ["nonexistent", false],
  ] as const)("isReservedPostgresRole(%j) is %s", (name, reserved) => {
    expect(isReservedPostgresRole(name)).toBe(reserved);
  });
});
