import React, { useMemo } from "react";
import { StyleProp, StyleSheet, View, ViewStyle } from "react-native";
import { ColorTheme } from "@/types/theme-props";
import { useThemeStyle } from "@/common/hooks/use-theme-style";

export const ListDivider = () => {
  const styles = useThemeStyle((theme: ColorTheme) =>
    StyleSheet.create({
      divider: {
        height: StyleSheet.hairlineWidth,
        backgroundColor: theme.black60,
        width: "100%",
      },
    }),
  );
  return <View style={styles.divider} />;
};

export const List = ({
  children,
  style,
}: {
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
}) => {
  const styles = useThemeStyle((theme: ColorTheme) =>
    StyleSheet.create({
      container: {
        borderBottomWidth: StyleSheet.hairlineWidth,
        borderBottomColor: theme.black20,
        borderTopWidth: StyleSheet.hairlineWidth,
        borderTopColor: theme.black20,
      },
    }),
  );

  // Convert children to array and add dividers between ListItem components
  const childrenArray = React.Children.toArray(children);
  const itemsWithDividers: React.ReactNode[] = [];

  childrenArray.forEach((child, index) => {
    // Add the child
    itemsWithDividers.push(child);

    // Add divider after each child except the last one
    if (index < childrenArray.length - 1) {
      itemsWithDividers.push(<ListDivider key={`divider-${index}`} />);
    }
  });

  const containerStyle = useMemo(
    () => StyleSheet.flatten([styles.container, style]),
    [style, styles.container],
  );

  return <View style={containerStyle}>{itemsWithDividers}</View>;
};
