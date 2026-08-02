import type {
  DeployActionError,
  DeployActionErrorCode,
} from "./deploy-action-types";

export type MutationDelivery = "not_sent" | "possibly_sent";

/** Transport adapters use this to say whether a failed request left the device. */
export class DeployMutationFailure extends Error {
  constructor(
    message: string,
    readonly delivery: MutationDelivery,
    readonly cause?: unknown,
  ) {
    super(message);
    this.name = "DeployMutationFailure";
  }
}

type ErrorShape = {
  message?: unknown;
  name?: unknown;
  status?: unknown;
  statusCode?: unknown;
  code?: unknown;
  networkError?: unknown;
  graphQLErrors?: unknown;
};

export function mapDeployActionError(error: unknown): DeployActionError {
  const failure = error instanceof DeployMutationFailure ? error : undefined;
  const source = failure?.cause ?? error;
  const delivery: MutationDelivery = failure?.delivery ?? "possibly_sent";
  const message =
    errorMessage(source) || failure?.message || "Deploy action failed.";
  const status = errorStatus(source);
  const lower = message.toLowerCase();

  let code: DeployActionErrorCode;
  if (
    status === 401 ||
    status === 403 ||
    lower.includes("unauthorized") ||
    lower.includes("forbidden")
  ) {
    code = "forbidden";
  } else if (
    status === 409 ||
    lower.includes("conflict") ||
    lower.includes("already terminal")
  ) {
    code = "conflict";
  } else if (status === 404 || lower.includes("not found")) code = "not_found";
  else if (status === 400 || lower.includes("bad request"))
    code = "invalid_request";
  else if (
    lower.includes("timeout") ||
    lower.includes("timed out") ||
    errorName(source) === "AbortError"
  ) {
    code = "timeout";
  } else if (
    status === 503 ||
    status === 429 ||
    (status !== undefined && status >= 500)
  ) {
    code = "unavailable";
  } else {
    code = "network";
  }

  const deterministic =
    code === "forbidden" ||
    code === "conflict" ||
    code === "not_found" ||
    code === "invalid_request";
  if (deterministic) {
    return {
      code,
      message,
      delivery: "rejected_by_server",
      refreshRequired: code === "conflict" || code === "not_found",
      retry:
        code === "conflict" || code === "not_found" ? "after_refresh" : "none",
    };
  }
  if (delivery === "possibly_sent") {
    return {
      code,
      message,
      delivery: "possibly_committed",
      refreshRequired: true,
      retry: "after_refresh",
    };
  }
  return {
    code,
    message,
    delivery: "not_sent",
    refreshRequired: false,
    retry: "safe",
  };
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error !== "object" || error === null) return "";
  const shape = error as ErrorShape;
  if (typeof shape.message === "string") return shape.message;
  if (Array.isArray(shape.graphQLErrors)) {
    const messages = shape.graphQLErrors
      .map((entry) => errorMessage(entry))
      .filter(Boolean);
    if (messages.length > 0) return messages.join("; ");
  }
  return errorMessage(shape.networkError);
}

function errorName(error: unknown): string {
  if (typeof error !== "object" || error === null) return "";
  const name = (error as ErrorShape).name;
  return typeof name === "string" ? name : "";
}

function errorStatus(error: unknown): number | undefined {
  if (typeof error !== "object" || error === null) return undefined;
  const shape = error as ErrorShape;
  for (const value of [shape.statusCode, shape.status, shape.code]) {
    if (typeof value === "number") return value;
    if (typeof value === "string" && /^\d{3}$/.test(value))
      return Number(value);
  }
  return errorStatus(shape.networkError);
}
