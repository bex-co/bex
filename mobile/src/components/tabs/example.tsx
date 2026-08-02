import React from "react";
import { View, Text, StyleSheet } from "react-native";
import { Tabs, TabItem } from "./index";
import { useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    container: {
      flex: 1,
      backgroundColor: theme.white,
    },
    content: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      padding: 20,
    },
    text: {
      fontSize: 18,
      color: theme.black,
      textAlign: "center",
    },
  });

export const TabsExample: React.FC = () => {
  const theme = useTheme().colorTheme;
  const styles = getStyles(theme);

  const tabs: TabItem[] = [
    {
      key: "tab1",
      title: "First Tab",
      component: (
        <View style={styles.content}>
          <Text style={styles.text}>This is the first tab content</Text>
        </View>
      ),
    },
    {
      key: "tab2",
      title: "Second Tab",
      component: (
        <View style={styles.content}>
          <Text style={styles.text}>This is the second tab content</Text>
        </View>
      ),
    },
    {
      key: "tab3",
      title: "Third Tab",
      component: (
        <View style={styles.content}>
          <Text style={styles.text}>This is the third tab content</Text>
        </View>
      ),
    },
    {
      key: "tab4",
      title: "Third Tab",
      component: (
        <View style={styles.content}>
          <Text style={styles.text}>This is the fourth tab content</Text>
        </View>
      ),
    },
    {
      key: "tab5",
      title: "Third Tab",
      component: (
        <View style={styles.content}>
          <Text style={styles.text}>This is the fifth tab content</Text>
        </View>
      ),
    },
  ];

  const handleTabChange = (index: number) => {
    console.log("Tab changed to index:", index);
  };

  return (
    <View style={styles.container}>
      <Tabs
        tabs={tabs}
        initialIndex={0}
        onTabChange={handleTabChange}
        scrollable={true}
        autoScrollToCenter={true}
      />
    </View>
  );
};

export default TabsExample;
