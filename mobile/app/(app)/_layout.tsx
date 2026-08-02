import { Redirect, Tabs } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@/common/theme";
import { useTranslations } from "@/common/hooks/use-translations";
import { HapticTab } from "@/components/haptic-tab";
import { AuthStateScreen } from "@/features/auth/auth-screen";
import { useAuth } from "@/features/auth/auth-provider";
import { useNotifications } from "@/features/notifications/notifications-provider";
import {
  useWorkspace,
  WorkspaceProvider,
} from "@/features/workspaces/workspace-provider";

function WorkspaceTabs() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { status, offline, switching, retry } = useWorkspace();
  const { unread } = useNotifications();
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
      />
    );
  }
  if (status === "empty") {
    return (
      <AuthStateScreen
        titleKey="workspace.emptyTitle"
        bodyKey="workspace.emptyBody"
      />
    );
  }
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarButton: HapticTab,
        tabBarActiveTintColor: theme.primary,
        tabBarInactiveTintColor: theme.mutedForeground,
        tabBarStyle: {
          backgroundColor: theme.card,
          borderTopColor: theme.border,
        },
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: t("navigation.status"),
          tabBarIcon: ({ color }) => (
            <Ionicons name="pulse" size={24} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="activity"
        options={{
          title: t("navigation.activity"),
          tabBarIcon: ({ color }) => (
            <Ionicons name="list" size={24} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="sessions"
        options={{
          title: t("navigation.sessions"),
          tabBarIcon: ({ color }) => (
            <Ionicons name="sparkles" size={24} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="notifications"
        options={{
          title: t("navigation.notifications"),
          tabBarBadge: unread > 0 ? unread : undefined,
          tabBarIcon: ({ color }) => (
            <Ionicons name="notifications" size={24} color={color} />
          ),
        }}
      />
      <Tabs.Screen name="services/[serviceId]" options={{ href: null }} />
      <Tabs.Screen name="services/[serviceId]/logs" options={{ href: null }} />
      <Tabs.Screen name="databases/[databaseId]" options={{ href: null }} />
      <Tabs.Screen name="key-values/[keyValueId]" options={{ href: null }} />
      <Tabs.Screen name="sessions/[sessionId]" options={{ href: null }} />
    </Tabs>
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
    <WorkspaceProvider>
      <WorkspaceTabs />
    </WorkspaceProvider>
  );
}
