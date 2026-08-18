import { useMemo, useState } from "react";
import {
  FlatList,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
  useWindowDimensions,
} from "react-native";
import {
  SafeAreaView,
  useSafeAreaInsets,
} from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  gutter,
  maxFontSizeMultipliers,
  rowMinHeight,
  space,
  useTheme,
} from "@/common/theme";

export type RepositoryPickerItem = {
  label: string;
  value: string;
};

export function RepositoryPicker({
  visible,
  items,
  selectedValue,
  onCancel,
  onSelect,
}: {
  visible: boolean;
  items: RepositoryPickerItem[];
  selectedValue?: string;
  onCancel: () => void;
  onSelect: (item: RepositoryPickerItem) => void;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { height } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const [query, setQuery] = useState("");
  const filteredItems = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    return normalized
      ? items.filter((item) =>
          item.label.toLocaleLowerCase().includes(normalized),
        )
      : items;
  }, [items, query]);

  const close = () => {
    setQuery("");
    onCancel();
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={close}
    >
      <KeyboardAvoidingView
        behavior={Platform.OS === "ios" ? "height" : undefined}
        style={[
          styles.overlay,
          {
            backgroundColor: theme.overlay,
            paddingTop: insets.top + space.sm,
          },
        ]}
      >
        <Pressable
          style={StyleSheet.absoluteFill}
          accessibilityRole="button"
          accessibilityLabel={t("common.cancel")}
          onPress={close}
        />
        <SafeAreaView
          edges={["bottom"]}
          style={[
            styles.sheet,
            { backgroundColor: theme.card, maxHeight: height * 0.72 },
          ]}
        >
          <View style={[styles.handle, { backgroundColor: theme.border }]} />
          <View style={styles.header}>
            <Text
              maxFontSizeMultiplier={maxFontSizeMultipliers.heading}
              style={[styles.title, { color: theme.foreground }]}
            >
              {t("agentSessions.composer.repo")}
            </Text>
            <Pressable
              testID="repository-picker-close"
              accessibilityRole="button"
              accessibilityLabel={t("common.cancel")}
              onPress={close}
              style={styles.close}
            >
              <Ionicons name="close" size={22} color={theme.foreground} />
            </Pressable>
          </View>
          <View
            style={[
              styles.search,
              { backgroundColor: theme.background, borderColor: theme.border },
            ]}
          >
            <Ionicons name="search" size={18} color={theme.mutedForeground} />
            <TextInput
              testID="repository-picker-search"
              value={query}
              onChangeText={setQuery}
              placeholder={t("agentSessions.composer.searchRepos")}
              placeholderTextColor={theme.mutedForeground}
              autoCapitalize="none"
              autoCorrect={false}
              returnKeyType="search"
              maxFontSizeMultiplier={maxFontSizeMultipliers.control}
              style={[styles.searchInput, { color: theme.foreground }]}
            />
            {query ? (
              <Pressable
                accessibilityRole="button"
                accessibilityLabel={t("agentSessions.composer.clearRepoSearch")}
                onPress={() => setQuery("")}
                hitSlop={8}
              >
                <Ionicons
                  name="close-circle"
                  size={18}
                  color={theme.mutedForeground}
                />
              </Pressable>
            ) : null}
          </View>
          <FlatList
            testID="repository-picker-list"
            data={filteredItems}
            keyExtractor={(item) => item.value}
            keyboardShouldPersistTaps="handled"
            contentContainerStyle={
              filteredItems.length === 0 ? styles.emptyList : undefined
            }
            renderItem={({ item, index }) => {
              const selected = item.value === selectedValue;
              return (
                <Pressable
                  testID={`repository-picker-item-${item.value}`}
                  accessibilityRole="radio"
                  accessibilityLabel={item.label}
                  accessibilityState={{ selected }}
                  onPress={() => {
                    setQuery("");
                    onSelect(item);
                  }}
                  style={({ pressed }) => [
                    styles.row,
                    index > 0 && {
                      borderTopWidth: StyleSheet.hairlineWidth,
                      borderTopColor: theme.border,
                    },
                    selected && { backgroundColor: theme.primaryMuted },
                    pressed && styles.pressed,
                  ]}
                >
                  <Ionicons
                    name="git-branch-outline"
                    size={18}
                    color={selected ? theme.primary : theme.mutedForeground}
                  />
                  <Text
                    numberOfLines={1}
                    ellipsizeMode="middle"
                    maxFontSizeMultiplier={maxFontSizeMultipliers.control}
                    style={[
                      styles.label,
                      { color: selected ? theme.primary : theme.foreground },
                    ]}
                  >
                    {item.label}
                  </Text>
                  {selected ? (
                    <Ionicons
                      name="checkmark"
                      size={20}
                      color={theme.primary}
                    />
                  ) : null}
                </Pressable>
              );
            }}
            ListEmptyComponent={
              <Text
                maxFontSizeMultiplier={maxFontSizeMultipliers.content}
                style={[styles.empty, { color: theme.mutedForeground }]}
              >
                {t("agentSessions.composer.noRepoMatches")}
              </Text>
            }
          />
        </SafeAreaView>
      </KeyboardAvoidingView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: { flex: 1, justifyContent: "flex-end" },
  sheet: {
    flexShrink: 1,
    borderTopLeftRadius: space.xl,
    borderTopRightRadius: space.xl,
    overflow: "hidden",
    paddingTop: space.sm,
  },
  handle: {
    alignSelf: "center",
    width: space.xxl,
    height: space.xs,
    borderRadius: space.xs,
  },
  header: {
    minHeight: 52,
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: gutter,
    gap: space.md,
  },
  title: {
    flex: 1,
    fontSize: fontSizes.xl,
    fontWeight: fontWeights.medium,
  },
  close: {
    minWidth: rowMinHeight,
    minHeight: rowMinHeight,
    alignItems: "center",
    justifyContent: "center",
  },
  search: {
    minHeight: rowMinHeight,
    marginHorizontal: gutter,
    marginBottom: space.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: space.sm,
    paddingHorizontal: space.md,
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
  },
  searchInput: { flex: 1, paddingVertical: space.sm, fontSize: fontSizes.md },
  row: {
    minHeight: 52,
    paddingHorizontal: gutter,
    paddingVertical: space.md,
    flexDirection: "row",
    alignItems: "center",
    gap: space.md,
  },
  label: { flex: 1, fontSize: fontSizes.md },
  pressed: { opacity: 0.65 },
  emptyList: { flexGrow: 1, justifyContent: "center" },
  empty: {
    padding: gutter,
    textAlign: "center",
    fontSize: fontSizes.sm,
  },
});
