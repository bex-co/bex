// Pure draft/validation/error logic for the workspace agent environment
// (w10/m17/t005). The settings card owns only rendering and state; every
// decision about what is valid, what changed and which field a server refusal
// belongs to lives here so it is unit-testable without React.

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  AgentNetworkTier,
  type AgentEnvironmentFieldsFragment,
  type AgentEnvironmentInput,
} from "@/graphql/definitions";

export type AgentEnvironmentView = AgentEnvironmentFieldsFragment;

/** The custom-host cap the backend enforces (sessionegress maxExtraDestinations). */
export const MAX_CUSTOM_HOSTS = 32;

/**
 * The model-id shape the backend accepts (agentsessions modelIDPattern):
 * provider ids and OpenRouter slugs, never whitespace or shell syntax.
 */
const MODEL_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._/:@~+[\]-]{0,254}$/;

// A DNS-1123 subdomain with at least two labels: what ExtraDestinations admits
// before its private/cluster-local and baseline checks (those stay server-side).
const DNS_LABEL = "[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?";
const HOSTNAME_PATTERN = new RegExp(`^${DNS_LABEL}(\\.${DNS_LABEL})+$`);
const IPV4_PATTERN = /^\d{1,3}(\.\d{1,3}){3}$/;

/** The editable fields, as the form holds them. */
export interface AgentEnvironmentDraft {
  networkTier: AgentNetworkTier;
  /** One hostname per line, as typed. */
  hostsText: string;
  defaultModel: string;
}

export function draftFrom(env: AgentEnvironmentView): AgentEnvironmentDraft {
  return {
    networkTier: env.networkTier,
    hostsText: env.customHosts.join("\n"),
    defaultModel: env.defaultModel ?? "",
  };
}

/** The textarea's hostnames: trimmed, lowercased, blank lines dropped. */
export function parseHostLines(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim().toLowerCase())
    .filter((line) => line !== "");
}

/**
 * A rejected host list, with the same `reason` vocabulary the backend's
 * AGENT_ENVIRONMENT_HOSTS_INVALID carries, so client and server errors share
 * one set of messages.
 */
export interface HostsProblem {
  reason: string;
  entry?: string;
}

/** The first problem with a custom host list, or null when it is acceptable. */
export function validateCustomHosts(hosts: string[]): HostsProblem | null {
  if (hosts.length > MAX_CUSTOM_HOSTS) return { reason: "too_many_entries" };
  const seen = new Set<string>();
  for (const host of hosts) {
    if (/[:/*?#@]/.test(host)) return { reason: "not_a_hostname", entry: host };
    if (IPV4_PATTERN.test(host)) return { reason: "ip_literal", entry: host };
    if (host.length > 253 || !HOSTNAME_PATTERN.test(host)) {
      return { reason: "not_a_hostname", entry: host };
    }
    if (seen.has(host)) return { reason: "duplicate", entry: host };
    seen.add(host);
  }
  return null;
}

/** Whether a default-model value is acceptable ("" = provider default). */
export function validModel(model: string): boolean {
  return model === "" || MODEL_ID_PATTERN.test(model);
}

function sameHosts(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const left = [...a].sort();
  const right = [...b].sort();
  return left.every((host, i) => host === right[i]);
}

/**
 * The partial update the draft implies: only the fields that differ from the
 * saved environment. Custom hosts are sent only while the Custom tier is
 * selected (they are hidden and inert otherwise), and the model never while it
 * is pinned. An empty object means nothing to save.
 */
export function buildPatch(
  saved: AgentEnvironmentView,
  draft: AgentEnvironmentDraft,
): AgentEnvironmentInput {
  const patch: AgentEnvironmentInput = {};
  if (draft.networkTier !== saved.networkTier) {
    patch.networkTier = draft.networkTier;
  }
  if (draft.networkTier === AgentNetworkTier.Custom) {
    const hosts = parseHostLines(draft.hostsText);
    if (!sameHosts(hosts, saved.customHosts)) patch.customHosts = hosts;
  }
  if (!saved.modelPinned) {
    const model = draft.defaultModel.trim();
    if (model !== (saved.defaultModel ?? "")) patch.defaultModel = model;
  }
  return patch;
}

/** Client-side problems with a draft, keyed by field. */
export interface DraftProblems {
  hosts: HostsProblem | null;
  model: boolean;
}

export function draftProblems(draft: AgentEnvironmentDraft): DraftProblems {
  return {
    hosts:
      draft.networkTier === AgentNetworkTier.Custom
        ? validateCustomHosts(parseHostLines(draft.hostsText))
        : null,
    model: !validModel(draft.defaultModel.trim()),
  };
}

/** A save refusal, anchored to the field it belongs to when it has one. */
export type SaveError =
  | { field: "hosts"; problem: HostsProblem }
  | { field: "model"; key: string; params?: Record<string, string> }
  | { field: "form"; key: string; params?: Record<string, string> };

/**
 * Maps a failed save onto a field by the server's stable `extensions.code`,
 * never its wording. Unknown failures become a form-level error with the
 * server's message as the fallback.
 */
export function saveErrorFrom(err: unknown): SaveError {
  if (CombinedGraphQLErrors.is(err)) {
    for (const item of err.errors) {
      const ext = (item.extensions ?? {}) as Record<string, unknown>;
      const str = (key: string) =>
        typeof ext[key] === "string" ? (ext[key] as string) : undefined;
      switch (ext["code"]) {
        case "AGENT_ENVIRONMENT_HOSTS_INVALID":
          return {
            field: "hosts",
            problem: {
              reason: str("reason") ?? "invalid",
              entry: str("entry"),
            },
          };
        case "AGENT_ENVIRONMENT_MODEL_INVALID":
          return { field: "model", key: "agentEnvironment.modelInvalid" };
        case "AGENT_ENVIRONMENT_MODEL_PINNED":
          return {
            field: "model",
            key: "agentEnvironment.modelPinnedError",
            params: { model: str("pinnedModel") ?? "" },
          };
        case "AGENT_ENVIRONMENT_TIER_INVALID":
          return { field: "form", key: "agentEnvironment.tierInvalid" };
        case "FORBIDDEN":
          return { field: "form", key: "agentEnvironment.forbidden" };
      }
    }
    const first = err.errors[0];
    if (first && first.extensions?.["code"] !== "INTERNAL" && first.message) {
      return {
        field: "form",
        key: "agentEnvironment.saveFailedReason",
        params: { reason: first.message },
      };
    }
  }
  return { field: "form", key: "agentEnvironment.saveFailed" };
}

const HOST_REASONS = new Set([
  "too_many_entries",
  "empty",
  "whitespace",
  "not_a_hostname",
  "ip_literal",
  "private_name",
  "already_baseline",
  "duplicate",
]);

/** The locale key + params that describe a host-list problem. */
export function hostsProblemMessage(problem: HostsProblem): {
  key: string;
  params: Record<string, string | number>;
} {
  const reason = HOST_REASONS.has(problem.reason) ? problem.reason : "invalid";
  return {
    key: `agentEnvironment.hostsError.${reason}`,
    params: { entry: problem.entry ?? "", max: MAX_CUSTOM_HOSTS },
  };
}
