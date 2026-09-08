import { Redirect, Stack } from "expo-router";
import { useTheme } from "@/common/theme";
import { AuthStateScreen } from "@/features/auth/auth-screen";
import { useAuth } from "@/features/auth/auth-provider";
import {
  useWorkspace,
  WorkspaceProvider,
} from "@/features/workspaces/workspace-provider";
import { AppDrawerProvider } from "@/components/app-drawer";
import {
  CapabilitiesProvider,
  useCapabilities,
} from "@/features/capabilities/capabilities-provider";
import { NotificationsProvider } from "@/features/notifications/notifications-provider";
import { WorkspaceList } from "@/components/app-drawer/workspace-list";
import { SafeAreaView } from "react-native-safe-area-context";

// Deep links to a detail still get a tab screen to return to.
export const unstable_settings = { initialRouteName: "(tabs)" };

function WorkspaceStack() {
  const theme = useTheme().colorTheme;
  const { status, offline, switching, retry } = useWorkspace();
  const { signOut } = useAuth();
  const { generation } = useCapabilities();
  // A failed or empty workspace load is otherwise a dead end (no drawer behind
  // the gate), so both offer sign-out — the only way to mint a fresh token when
  // the current session lacks capability scope (w11 auth contract).
  const signOutAction = () => void signOut().catch(() => undefined);
  if (status === "loading" || switching) {
    return (
      <AuthStateScreen
        titleKey={
          switching ? "workspace.switchingTitle" : "workspace.loadingTitle"
        }
        bodyKey={
          switching ? "workspace.switchingBody" : "workspace.loadingBody"
        }
        busy
      />
    );
  }
  if (status === "error") {
    return (
      <AuthStateScreen
        titleKey={offline ? "workspace.offlineTitle" : "workspace.errorTitle"}
        bodyKey={offline ? "workspace.offlineBody" : "workspace.errorBody"}
        actionKey="auth.retry"
        onAction={() => void retry().catch(() => undefined)}
        secondaryActionKey="auth.signOut"
        onSecondaryAction={signOutAction}
      />
    );
  }
  if (status === "empty") {
    return (
      <AuthStateScreen
        titleKey="workspace.emptyTitle"
        bodyKey="workspace.emptyBody"
        secondaryActionKey="auth.signOut"
        onSecondaryAction={signOutAction}
      />
    );
  }
  if (status === "choose") {
    return (
      <SafeAreaView style={{ flex: 1, backgroundColor: theme.background }}>
        <WorkspaceList onSelected={() => undefined} />
      </SafeAreaView>
    );
  }
  return (
    <Stack
      key={generation}
      initialRouteName="(tabs)"
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: theme.background },
      }}
    >
      <Stack.Screen name="(tabs)" />
    </Stack>
  );
}

export default function AppLayout() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <AuthStateScreen
        titleKey="auth.loadingTitle"
        bodyKey="auth.loadingBody"
        busy
      />
    );
  }
  if (state.status !== "signedIn") return <Redirect href="/sign-in" />;
  return (
    <WorkspaceProvider key={state.session.sessionId}>
      <CapabilitiesProvider>
        <NotificationsProvider>
          <AppDrawerProvider>
            <WorkspaceStack />
          </AppDrawerProvider>
        </NotificationsProvider>
      </CapabilitiesProvider>
    </WorkspaceProvider>
  );
}
