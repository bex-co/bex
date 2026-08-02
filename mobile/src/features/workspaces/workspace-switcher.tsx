import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { Button } from "@/components/button";
import { Picker } from "@/components/picker";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/theme";
import { useWorkspace } from "./workspace-provider";

export function WorkspaceSwitcher() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { status, workspaces, selected, switching, offline, switchWorkspace } =
    useWorkspace();
  const [visible, setVisible] = useState(false);
  if (status !== "ready" || !selected) return null;
  return (
    <View style={styles.container}>
      <Text style={[styles.label, { color: theme.mutedForeground }]}>
        {t("workspace.current")}
      </Text>
      <Button
        type="outline"
        disabled={switching}
        onPress={() => setVisible(true)}
        accessibilityLabel={t("workspace.switchLabel", {
          name: selected.name,
        })}
      >
        {selected.name}
      </Button>
      {offline ? (
        <Text style={{ color: theme.mutedForeground }}>
          {t("workspace.offline")}
        </Text>
      ) : null}
      <Picker
        visible={visible}
        items={workspaces.map(({ id, name }) => ({ value: id, label: name }))}
        selectedValue={selected.id}
        title={t("workspace.choose")}
        confirmButtonText={t("workspace.confirm")}
        cancelButtonText={t("workspace.cancel")}
        onCancel={() => setVisible(false)}
        onSelect={(item) => {
          void switchWorkspace(item.value).catch(() => undefined);
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { gap: 8, width: "100%" },
  label: { fontSize: 13, fontWeight: "600" },
});
