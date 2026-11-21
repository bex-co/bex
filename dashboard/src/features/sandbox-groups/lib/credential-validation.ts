/**
 * Client-side validation and phrase helpers for sandbox group credentials
 * (ADR122, w2/m533). Mirrors `lego/backend/internal/sandbox/credentials_validate.go`
 * without ever being stricter than the server.
 */

export const CREDENTIAL_INJECTION_TYPES = [
  "header",
  "basic",
  "cookie",
  "query",
] as const;

export type CredentialInjectionType =
  (typeof CREDENTIAL_INJECTION_TYPES)[number];

export const CREDENTIAL_HTTP_METHODS = [
  "GET",
  "HEAD",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "OPTIONS",
] as const;

export const MAX_CREDENTIAL_VALUE_BYTES = 5120;
export const CREDENTIAL_TEMPLATE_PLACEHOLDER = "${SECRET}";
export const MAX_CREDENTIAL_TEMPLATE_PLACEHOLDERS = 5;

const CREDENTIAL_ID_RE = /^[A-Za-z0-9._~-]+$/;
const CREDENTIAL_DOMAIN_RE =
  /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$/;
const HTTP_TOKEN_RE = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/;

const FORBIDDEN_CREDENTIAL_HEADERS = new Set([
  "host",
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "proxy-connection",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
  "content-length",
  "forwarded",
  "via",
]);

const PRIVATE_OR_CLUSTER_SUFFIXES = [
  ".local",
  ".internal",
  ".localhost",
  ".svc",
  ".svc.cluster.local",
  ".cluster.local",
  ".home.arpa",
  ".arpa",
  ".localdomain",
  ".lan",
  ".corp",
  ".home",
];

export interface CredentialFormValues {
  id: string;
  domain: string;
  value: string;
  enabled: boolean;
  methods: string[];
  pathsText: string;
  injectionType: CredentialInjectionType;
  injectionName: string;
  injectionTemplate: string;
}

export type CredentialValidationErrorCode =
  | "idRequired"
  | "idInvalid"
  | "idReserved"
  | "domainRequired"
  | "domainInvalid"
  | "domainPrivate"
  | "valueRequired"
  | "valueTooLong"
  | "valueNul"
  | "injectionNameRequired"
  | "injectionNameForbidden"
  | "injectionHeaderForbidden"
  | "injectionNameInvalid"
  | "injectionTemplateCount"
  | "injectionBasicFormat"
  | "injectionHeaderControl"
  | "injectionCookieInvalid"
  | "pathsInvalid";

export interface CredentialValidationError {
  field:
    | "id"
    | "domain"
    | "value"
    | "injectionName"
    | "injectionTemplate"
    | "paths";
  code: CredentialValidationErrorCode;
}

export function isForbiddenCredentialHeader(name: string): boolean {
  const lower = name.toLowerCase();
  return (
    FORBIDDEN_CREDENTIAL_HEADERS.has(lower) ||
    lower.startsWith("x-forwarded-")
  );
}

export function parsePathsText(pathsText: string): string[] {
  return pathsText
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

export function isValidCredentialPathPattern(pattern: string): boolean {
  if (!pattern.startsWith("/")) return false;
  if (/[?#\\]/.test(pattern)) return false;
  if (/[\s\p{Cc}]/u.test(pattern)) return false;
  if (pattern === "/") return true;
  for (const segment of pattern.slice(1).split("/")) {
    if (segment.includes("*") && segment !== "*" && segment !== "**") {
      return false;
    }
  }
  return true;
}

function isPrivateOrNonPublicHost(host: string): boolean {
  if (
    host === "localhost" ||
    PRIVATE_OR_CLUSTER_SUFFIXES.some((suffix) => host.endsWith(suffix))
  ) {
    return true;
  }
  const lastDot = host.lastIndexOf(".");
  if (lastDot >= 0 && /^[0-9]+$/.test(host.slice(lastDot + 1))) {
    return true;
  }
  return false;
}

function utf8ByteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

function countPlaceholderOccurrences(template: string): number {
  let count = 0;
  let index = 0;
  while (true) {
    const next = template.indexOf(CREDENTIAL_TEMPLATE_PLACEHOLDER, index);
    if (next === -1) break;
    count += 1;
    index = next + CREDENTIAL_TEMPLATE_PLACEHOLDER.length;
  }
  return count;
}

export function validateCredentialForm(
  values: CredentialFormValues,
  options: { mode: "create" | "edit"; canManage: boolean },
): CredentialValidationError | null {
  if (options.mode === "create") {
    const id = values.id.trim();
    if (!id) return { field: "id", code: "idRequired" };
    if (!CREDENTIAL_ID_RE.test(id)) return { field: "id", code: "idInvalid" };
    if (id === "." || id === ".." || id.startsWith("rnd-")) {
      return { field: "id", code: "idReserved" };
    }
  }

  if (options.mode === "create" || options.canManage) {
    const rawDomain = values.domain.trim();
    if (!rawDomain) return { field: "domain", code: "domainRequired" };
    if (/[:/?#@[\]*\s]/.test(rawDomain)) {
      return { field: "domain", code: "domainInvalid" };
    }
    const domain = rawDomain.toLowerCase();
    if (domain.length > 253 || !CREDENTIAL_DOMAIN_RE.test(domain)) {
      return { field: "domain", code: "domainInvalid" };
    }
    if (isPrivateOrNonPublicHost(domain)) {
      return { field: "domain", code: "domainPrivate" };
    }
  }

  const hasValue = values.value.length > 0;
  if (options.mode === "create" && !hasValue) {
    return { field: "value", code: "valueRequired" };
  }
  if (hasValue) {
    if (utf8ByteLength(values.value) > MAX_CREDENTIAL_VALUE_BYTES) {
      return { field: "value", code: "valueTooLong" };
    }
    if (values.value.includes("\0")) {
      return { field: "value", code: "valueNul" };
    }
  }

  const injName = values.injectionName.trim();
  if (values.injectionType === "basic") {
    if (injName !== "") {
      return { field: "injectionName", code: "injectionNameForbidden" };
    }
  } else if (!injName) {
    return { field: "injectionName", code: "injectionNameRequired" };
  } else if (values.injectionType === "header") {
    if (!HTTP_TOKEN_RE.test(injName)) {
      return { field: "injectionName", code: "injectionNameInvalid" };
    }
    if (isForbiddenCredentialHeader(injName)) {
      return { field: "injectionName", code: "injectionHeaderForbidden" };
    }
  } else if (values.injectionType === "cookie") {
    if (!HTTP_TOKEN_RE.test(injName)) {
      return { field: "injectionName", code: "injectionNameInvalid" };
    }
  } else if (values.injectionType === "query") {
    if (/[\p{Cc}#]/u.test(injName)) {
      return { field: "injectionName", code: "injectionNameInvalid" };
    }
  }

  const template = values.injectionTemplate;
  if (template.length > 0) {
    const count = countPlaceholderOccurrences(template);
    if (count < 1 || count > MAX_CREDENTIAL_TEMPLATE_PLACEHOLDERS) {
      return { field: "injectionTemplate", code: "injectionTemplateCount" };
    }
  }

  if (hasValue) {
    const effectiveTemplate = template || CREDENTIAL_TEMPLATE_PLACEHOLDER;
    const rendered = effectiveTemplate
      .split(CREDENTIAL_TEMPLATE_PLACEHOLDER)
      .join(values.value);
    if (values.injectionType === "header" && /[\r\n\0]/.test(rendered)) {
      return { field: "value", code: "injectionHeaderControl" };
    }
    if (values.injectionType === "cookie" && /[\r\n\0;]/.test(rendered)) {
      return { field: "value", code: "injectionCookieInvalid" };
    }
    if (values.injectionType === "basic") {
      if (/[\r\n\0]/.test(rendered)) {
        return { field: "value", code: "injectionBasicFormat" };
      }
      const colonIdx = rendered.indexOf(":");
      if (colonIdx <= 0 || colonIdx === rendered.length - 1) {
        return { field: "value", code: "injectionBasicFormat" };
      }
    }
  }

  for (const path of parsePathsText(values.pathsText)) {
    if (!isValidCredentialPathPattern(path)) {
      return { field: "paths", code: "pathsInvalid" };
    }
  }

  return null;
}

export function sudoDeleteCredentialPhrase(credentialId: string): string {
  return `sudo delete credential ${credentialId}`;
}

export function sudoRestoreCredentialPhrase(credentialId: string): string {
  return `sudo restore credential ${credentialId}`;
}
