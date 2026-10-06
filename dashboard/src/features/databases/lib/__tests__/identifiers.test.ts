import { describe, expect, it } from "vitest";

import {
  isReservedPostgresDatabaseName,
  isReservedPostgresRole,
  isValidPostgresIdentifier,
} from "@/features/databases/lib/identifiers";
import { readRepoVectors, repeated } from "@/test/repo-vectors";

// The one table of Postgres identifiers. lego/types/v1alpha1's predicates are
// its source of truth (TestPostgresIdentifierVectors), and the operator's
// TestManagedRolesNeverProjectReservedRoles reads it too, so the form never
// offers a name bex-api refuses or CNPG cannot manage (w5/m118).
const VECTORS = readRepoVectors<{
  name: string;
  repeat?: number;
  valid: boolean;
  reservedRole: boolean;
  reservedDatabase: boolean;
}>("lego/types/v1alpha1/testdata/postgres-identifiers.json");

describe("the shared Postgres identifier vectors", () => {
  it.each(
    VECTORS.map(
      (v) =>
        [
          repeated(v.name, v.repeat),
          v.valid,
          v.reservedRole,
          v.reservedDatabase,
        ] as const,
    ),
  )(
    "%j: valid %s, reserved role %s, reserved database %s",
    (name, valid, reservedRole, reservedDatabase) => {
      expect(isValidPostgresIdentifier(name)).toBe(valid);
      expect(isReservedPostgresRole(name)).toBe(reservedRole);
      expect(isReservedPostgresDatabaseName(name)).toBe(reservedDatabase);
    },
  );
});
