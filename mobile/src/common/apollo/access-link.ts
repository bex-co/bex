import { ApolloLink } from "@apollo/client";
import { throwError } from "rxjs";
import { dataBoundary } from "./data-boundary";
import type { CapabilityAction } from "@/features/capabilities/capability-policy";

let checkAccess: (action: CapabilityAction) => boolean = () => false;

export function assertCurrentAccess(action: CapabilityAction): void {
  if (!checkAccess(action))
    throw new Error("mobile access is not currently verified");
}

export function installAccessCheck(check: typeof checkAccess): () => void {
  checkAccess = check;
  return () => {
    if (checkAccess === check) checkAccess = () => false;
  };
}

// Bootstrap and personal notification operations have their own server scope.
// They must remain usable before a workspace grant exists (including opt-out).
const independentOperations = new Set([
  "MobileWorkspaces",
  "MobileViewerCapabilities",
  "MobileAcceptWorkspaceInvite",
  "MobileNotificationDeviceSubscriptions",
  "MobileRegisterNotificationDeviceSubscription",
  "MobileUnregisterNotificationDeviceSubscription",
  "MobileNotificationInbox",
  "MobileMarkPushNotificationRead",
]);

export function createAccessLink() {
  return new ApolloLink((operation, forward) => {
    const name = operation.operationName ?? "";
    if (independentOperations.has(name)) return forward(operation);
    const action: CapabilityAction =
      operation.operationType === "mutation" || name.startsWith("MobileAgent")
        ? "can_operate"
        : "can_view";
    const creates =
      operation.operationName === "MobileCreateAgentSession" ||
      operation.operationName === "MobileRollbackService";
    if (
      operation.getContext().boundaryGeneration !==
        dataBoundary.getGeneration() ||
      !checkAccess(action) ||
      (creates && !checkAccess("can_create"))
    ) {
      return throwError(
        () => new Error("mobile access is not currently verified"),
      );
    }
    return forward(operation);
  });
}
