const serviceId = /^srv-[a-z0-9]+$/;
const databaseId = /^dpg-[a-z0-9]+$/;
const keyValueId = /^red-[a-z0-9]+$/;
const agentSessionId = /^ags-[a-z0-9]+$/;

export function validServiceDeepLink(value: unknown): value is string {
  return typeof value === "string" && serviceId.test(value);
}

export function validDatabaseDeepLink(value: unknown): value is string {
  return typeof value === "string" && databaseId.test(value);
}

export function validKeyValueDeepLink(value: unknown): value is string {
  return typeof value === "string" && keyValueId.test(value);
}

export function validAgentSessionDeepLink(value: unknown): value is string {
  return typeof value === "string" && agentSessionId.test(value);
}
