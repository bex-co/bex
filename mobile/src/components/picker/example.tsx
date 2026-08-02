import React, { useState } from "react";
import { View, Text, TouchableOpacity, StyleSheet } from "react-native";
import { Picker } from "./index";
import { useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    container: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      backgroundColor: theme.white,
    },
    button: {
      backgroundColor: theme.primary,
      paddingHorizontal: 20,
      paddingVertical: 12,
      borderRadius: 8,
    },
    buttonText: {
      color: theme.white,
      fontSize: 16,
      fontWeight: "600",
    },
    selectedText: {
      marginTop: 20,
      fontSize: 18,
      color: theme.text01,
    },
  });

const sampleItems = [
  { label: "Option 1", value: "option1" },
  { label: "Option 2", value: "option2" },
  { label: "Option 3", value: "option3" },
  { label: "Option 4", value: "option4" },
  { label: "Option 5", value: "option5" },
  { label: "Option 6", value: "option6" },
  { label: "Option 7", value: "option7" },
  { label: "Option 8", value: "option8" },
  { label: "Option 9", value: "option9" },
  { label: "Option 10", value: "option10" },
];

export const PickerExample: React.FC = () => {
  const [visible, setVisible] = useState(false);
  const [selectedItem, setSelectedItem] = useState<{
    label: string;
    value: string;
  } | null>(null);
  const theme = useTheme().colorTheme;
  const styles = getStyles(theme);

  const handleSelect = (item: { label: string; value: string }) => {
    setSelectedItem(item);
    setVisible(false);
  };

  const handleCancel = () => {
    setVisible(false);
  };

  return (
    <View style={styles.container}>
      <TouchableOpacity style={styles.button} onPress={() => setVisible(true)}>
        <Text style={styles.buttonText}>Open Picker</Text>
      </TouchableOpacity>

      {selectedItem && (
        <Text style={styles.selectedText}>
          Selected: {selectedItem.label} ({selectedItem.value})
        </Text>
      )}

      <Picker
        visible={visible}
        items={sampleItems}
        onSelect={handleSelect}
        onCancel={handleCancel}
        selectedValue={selectedItem?.value}
      />
    </View>
  );
};
