const serviceId = /^srv-[a-z0-9]+$/;
const agentSessionId = /^ags-[a-z0-9]+$/;

export function validServiceDeepLink(value: unknown): value is string {
  return typeof value === "string" && serviceId.test(value);
}

export function validAgentSessionDeepLink(value: unknown): value is string {
  return typeof value === "string" && agentSessionId.test(value);
}
