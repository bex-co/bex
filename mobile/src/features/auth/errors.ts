export type AuthFailureCode =
  | "cancelled"
  | "configuration"
  | "discovery"
  | "invalid_grant"
  | "invalid_redirect"
  | "invalid_response"
  | "offline"
  | "replay"
  | "storage"
  | "unavailable";

export class AuthFailure extends Error {
  constructor(
    readonly code: AuthFailureCode,
    message = code,
  ) {
    super(message);
    this.name = "AuthFailure";
  }
}

export function authFailureCode(error: unknown): AuthFailureCode {
  if (error instanceof AuthFailure) return error.code;
  if (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    error.code === "ERR_NETWORK"
  ) {
    return "offline";
  }
  return "unavailable";
}
