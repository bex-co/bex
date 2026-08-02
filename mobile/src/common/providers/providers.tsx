import type { ReactNode } from "react";
import { StyleSheet } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { ThemeProvider } from "./theme-provider";
import { LanguageProvider } from "./language-provider";
import { AuthProvider } from "@/features/auth/auth-provider";
import { BexApolloProvider } from "@/common/apollo/apollo-provider";
import { NetworkStateProvider } from "@/common/apollo/network-state";
import { NotificationsProvider } from "@/features/notifications/notifications-provider";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <GestureHandlerRootView style={styles.fill}>
      <SafeAreaProvider>
        <LanguageProvider>
          <ThemeProvider>
            <NetworkStateProvider>
              <AuthProvider>
                <BexApolloProvider>
                  <NotificationsProvider>{children}</NotificationsProvider>
                </BexApolloProvider>
              </AuthProvider>
            </NetworkStateProvider>
          </ThemeProvider>
        </LanguageProvider>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({ fill: { flex: 1 } });
