import { useEffect, useMemo, useState } from "react";
import {
  FlatList,
  Modal,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";
import { filterItems, type SearchableItem } from "./filter";

export type { SearchableItem };

type Props = {
  visible: boolean;
  title: string;
  items: SearchableItem[];
  selectedValue?: string;
  searchPlaceholder: string;
  cancelLabel: string;
  emptyLabel: string;
  onSelect: (value: string) => void;
  onCancel: () => void;
};

/**
 * A bottom-sheet list with a live substring filter — the searchable counterpart
 * to the wheel {@link Picker}, for long lists (e.g. repositories). Selection is
 * a single row tap; the query resets each time the sheet opens.
 */
export function SearchableListModal({
  visible,
  title,
  items,
  selectedValue,
  searchPlaceholder,
  cancelLabel,
  emptyLabel,
  onSelect,
  onCancel,
}: Props) {
  const theme = useTheme().colorTheme;
  const styles = getStyles(theme);
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (visible) setQuery("");
  }, [visible]);

  const filtered = useMemo(() => filterItems(items, query), [items, query]);

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={onCancel}
    >
      <View style={styles.overlay}>
        <Pressable
          style={styles.backdrop}
          onPress={onCancel}
          accessibilityRole="button"
          accessibilityLabel={cancelLabel}
        />
        <View style={styles.sheet}>
          <View style={styles.header}>
            <Pressable
              onPress={onCancel}
              hitSlop={8}
              accessibilityRole="button"
              accessibilityLabel={cancelLabel}
            >
              <Text style={styles.cancel}>{cancelLabel}</Text>
            </Pressable>
            <Text style={styles.title} numberOfLines={1}>
              {title}
            </Text>
            <View style={styles.headerSpacer} />
          </View>

          <View style={styles.searchBox}>
            <Ionicons name="search" size={16} color={theme.mutedForeground} />
            <TextInput
              value={query}
              onChangeText={setQuery}
              placeholder={searchPlaceholder}
              placeholderTextColor={theme.mutedForeground}
              autoCapitalize="none"
              autoCorrect={false}
              autoFocus
              style={styles.searchInput}
            />
            {query !== "" ? (
              <Pressable
                onPress={() => setQuery("")}
                hitSlop={8}
                accessibilityRole="button"
                accessibilityLabel={cancelLabel}
              >
                <Ionicons
                  name="close-circle"
                  size={16}
                  color={theme.mutedForeground}
                />
              </Pressable>
            ) : null}
          </View>

          <FlatList
            data={filtered}
            keyExtractor={(item) => item.value}
            keyboardShouldPersistTaps="handled"
            keyboardDismissMode="on-drag"
            ListEmptyComponent={<Text style={styles.empty}>{emptyLabel}</Text>}
            renderItem={({ item }) => {
              const selected = item.value === selectedValue;
              return (
                <Pressable
                  onPress={() => onSelect(item.value)}
                  accessibilityRole="button"
                  accessibilityState={{ selected }}
                  accessibilityLabel={item.label}
                  style={styles.row}
                >
                  <Text
                    numberOfLines={1}
                    style={[
                      styles.rowText,
                      selected && { color: theme.primary },
                    ]}
                  >
                    {item.label}
                  </Text>
                  {selected ? (
                    <Ionicons
                      name="checkmark"
                      size={18}
                      color={theme.primary}
                    />
                  ) : null}
                </Pressable>
              );
            }}
          />
        </View>
      </View>
    </Modal>
  );
}

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    overlay: {
      flex: 1,
      backgroundColor: theme.overlay,
      justifyContent: "flex-end",
    },
    backdrop: { flex: 1 },
    sheet: {
      maxHeight: "85%",
      backgroundColor: theme.white,
      borderTopLeftRadius: 16,
      borderTopRightRadius: 16,
      paddingBottom: 16,
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      paddingHorizontal: 16,
      paddingVertical: 14,
      borderBottomWidth: StyleSheet.hairlineWidth,
      borderBottomColor: theme.black40,
    },
    cancel: { color: theme.black80, fontSize: 16, width: 60 },
    title: { fontSize: 18, fontWeight: "600", color: theme.text01 },
    headerSpacer: { width: 60 },
    searchBox: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      margin: 16,
      paddingHorizontal: 12,
      height: 40,
      borderRadius: 10,
      backgroundColor: theme.black10,
    },
    searchInput: { flex: 1, fontSize: 16, color: theme.text01 },
    row: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      paddingHorizontal: 16,
      paddingVertical: 14,
      borderBottomWidth: StyleSheet.hairlineWidth,
      borderBottomColor: theme.black10,
    },
    rowText: { fontSize: 16, color: theme.text01, flexShrink: 1 },
    empty: {
      textAlign: "center",
      color: theme.black80,
      fontSize: 14,
      paddingVertical: 32,
    },
  });
