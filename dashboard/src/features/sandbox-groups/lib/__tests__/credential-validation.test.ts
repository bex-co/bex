import { describe, expect, it } from "vitest";
import {
  isForbiddenCredentialHeader,
  isValidCredentialPathPattern,
  MAX_CREDENTIAL_VALUE_BYTES,
  parsePathsText,
  sudoDeleteCredentialPhrase,
  sudoRestoreCredentialPhrase,
  validateCredentialForm,
  type CredentialFormValues,
} from "@/features/sandbox-groups/lib/credential-validation";

const BASE: CredentialFormValues = {
  id: "openai-key",
  domain: "api.openai.com",
  value: "sk-live-123",
  enabled: true,
  methods: ["POST"],
  pathsText: "/v1/**\n/v2/models/*",
  injectionType: "header",
  injectionName: "Authorization",
  injectionTemplate: "Bearer ${SECRET}",
};

describe("credential-validation", () => {
  it("accepts a valid create form", () => {
    expect(
      validateCredentialForm(BASE, { mode: "create", canManage: true }),
    ).toBeNull();
  });

  it("validates credential IDs including reserved names", () => {
    expect(
      validateCredentialForm(
        { ...BASE, id: "" },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "id", code: "idRequired" });

    for (const bad of ["bad/id", "has space", "a:b"]) {
      expect(
        validateCredentialForm(
          { ...BASE, id: bad },
          { mode: "create", canManage: true },
        ),
      ).toEqual({ field: "id", code: "idInvalid" });
    }

    for (const reserved of [".", "..", "rnd-abc"]) {
      expect(
        validateCredentialForm(
          { ...BASE, id: reserved },
          { mode: "create", canManage: true },
        ),
      ).toEqual({ field: "id", code: "idReserved" });
    }
  });

  it("rejects private, cluster, wildcard, and malformed domains", () => {
    for (const bad of [
      "*.openai.com",
      "https://api.openai.com",
      "api.openai.com:443",
      "singlelabel",
    ]) {
      expect(
        validateCredentialForm(
          { ...BASE, domain: bad },
          { mode: "create", canManage: true },
        ),
      ).toEqual({ field: "domain", code: "domainInvalid" });
    }

    for (const priv of [
      "api.local",
      "db.internal",
      "bex-api.bex-system.svc.cluster.local",
      "127.0.0.1",
    ]) {
      expect(
        validateCredentialForm(
          { ...BASE, domain: priv },
          { mode: "create", canManage: true },
        ),
      ).toEqual({ field: "domain", code: "domainPrivate" });
    }
  });

  it("enforces secret presence on create, optional on edit, and 5 KiB UTF-8 cap", () => {
    expect(
      validateCredentialForm(
        { ...BASE, value: "" },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "value", code: "valueRequired" });

    expect(
      validateCredentialForm(
        { ...BASE, value: "" },
        { mode: "edit", canManage: true },
      ),
    ).toBeNull();

    expect(
      validateCredentialForm(
        { ...BASE, value: "x".repeat(MAX_CREDENTIAL_VALUE_BYTES + 1) },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "value", code: "valueTooLong" });

    expect(
      validateCredentialForm(
        { ...BASE, value: "bad\0nul" },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "value", code: "valueNul" });
  });

  it("enforces header, basic, cookie, query, and ${SECRET} template rules", () => {
    expect(isForbiddenCredentialHeader("Host")).toBe(true);
    expect(isForbiddenCredentialHeader("X-Forwarded-For")).toBe(true);
    expect(isForbiddenCredentialHeader("Authorization")).toBe(false);

    expect(
      validateCredentialForm(
        { ...BASE, injectionName: "Host" },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "injectionName", code: "injectionHeaderForbidden" });

    expect(
      validateCredentialForm(
        { ...BASE, injectionType: "basic", injectionName: "Authorization", value: "u:p" },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "injectionName", code: "injectionNameForbidden" });

    expect(
      validateCredentialForm(
        {
          ...BASE,
          injectionType: "basic",
          injectionName: "",
          injectionTemplate: "",
          value: "missing-colon",
        },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "value", code: "injectionBasicFormat" });

    expect(
      validateCredentialForm(
        {
          ...BASE,
          injectionType: "basic",
          injectionName: "",
          injectionTemplate: "alice:${SECRET}",
          value: "secret-pass",
        },
        { mode: "create", canManage: true },
      ),
    ).toBeNull();

    expect(
      validateCredentialForm(
        { ...BASE, injectionTemplate: "no-placeholder" },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "injectionTemplate", code: "injectionTemplateCount" });

    expect(
      validateCredentialForm(
        { ...BASE, injectionTemplate: "${SECRET}".repeat(6) },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "injectionTemplate", code: "injectionTemplateCount" });

    expect(
      validateCredentialForm(
        {
          ...BASE,
          injectionType: "cookie",
          injectionName: "session",
          injectionTemplate: "",
          value: "abc;evil=1",
        },
        { mode: "create", canManage: true },
      ),
    ).toEqual({ field: "value", code: "injectionCookieInvalid" });
  });

  it("validates path patterns and sudo phrases", () => {
    expect(parsePathsText("/v1/**\n\n  /v2/*  ")).toEqual(["/v1/**", "/v2/*"]);
    expect(isValidCredentialPathPattern("/")).toBe(true);
    expect(isValidCredentialPathPattern("/v1/*/items/**")).toBe(true);
    expect(isValidCredentialPathPattern("no-leading-slash")).toBe(false);
    expect(isValidCredentialPathPattern("/v1/a?b=1")).toBe(false);
    expect(isValidCredentialPathPattern("/v1/foo*")).toBe(false);

    expect(sudoDeleteCredentialPhrase("openai-key")).toBe(
      "sudo delete credential openai-key",
    );
    expect(sudoRestoreCredentialPhrase("openai-key")).toBe(
      "sudo restore credential openai-key",
    );
  });
});
