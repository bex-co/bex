import { View, ViewProps, StyleSheet } from "react-native";

export const FlexCenter = ({ children, ...props }: ViewProps) => {
  return (
    <View style={styles.flex} {...props}>
      {children}
    </View>
  );
};

const styles = StyleSheet.create({
  flex: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
});
