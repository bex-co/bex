import { createContext, useContext, type ReactNode } from "react";
import { useNetInfo } from "@react-native-community/netinfo";

export type MobileNetworkState = "checking" | "online" | "offline";

const NetworkContext = createContext<MobileNetworkState>("checking");

export function NetworkStateProvider({ children }: { children: ReactNode }) {
  const network = useNetInfo();
  const state: MobileNetworkState =
    network.isConnected === false || network.isInternetReachable === false
      ? "offline"
      : network.isConnected === true
        ? "online"
        : "checking";
  return (
    <NetworkContext.Provider value={state}>{children}</NetworkContext.Provider>
  );
}

export function useNetworkState(): MobileNetworkState {
  return useContext(NetworkContext);
}
