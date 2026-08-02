# Tabs Component

A modern, customizable tabs component built with `react-native-pager-view` for smooth page transitions and native performance.

## Features

- ✅ Native performance with `react-native-pager-view`
- ✅ Smooth page transitions
- ✅ Customizable styling
- ✅ Theme integration
- ✅ Scrollable tab bar support
- ✅ TypeScript support
- ✅ Callback for tab changes
- ✅ Active/inactive tab styling

## Installation

The component uses `react-native-pager-view` which is already installed in this project.

## Basic Usage

```tsx
import React from "react";
import { View, Text } from "react-native";
import { Tabs, TabItem } from "@/components/tabs";

const MyComponent = () => {
  const tabs: TabItem[] = [
    {
      key: "tab1",
      title: "First Tab",
      component: (
        <View
          style={{ flex: 1, justifyContent: "center", alignItems: "center" }}
        >
          <Text>First tab content</Text>
        </View>
      ),
    },
    {
      key: "tab2",
      title: "Second Tab",
      component: (
        <View
          style={{ flex: 1, justifyContent: "center", alignItems: "center" }}
        >
          <Text>Second tab content</Text>
        </View>
      ),
    },
  ];

  return (
    <Tabs
      tabs={tabs}
      initialIndex={0}
      onTabChange={(index) => console.log("Tab changed:", index)}
    />
  );
};
```

## Props

### TabsProps

| Prop                    | Type                      | Default      | Description                                              |
| ----------------------- | ------------------------- | ------------ | -------------------------------------------------------- |
| `tabs`                  | `TabItem[]`               | **required** | Array of tab items                                       |
| `initialIndex`          | `number`                  | `0`          | Initial active tab index                                 |
| `onTabChange`           | `(index: number) => void` | -            | Callback when tab changes                                |
| `tabBarStyle`           | `ViewStyle`               | -            | Custom style for tab bar                                 |
| `tabStyle`              | `ViewStyle`               | -            | Custom style for all tabs                                |
| `activeTabStyle`        | `ViewStyle`               | -            | Custom style for active tab                              |
| `inactiveTabStyle`      | `ViewStyle`               | -            | Custom style for inactive tabs                           |
| `tabTextStyle`          | `TextStyle`               | -            | Custom style for tab text                                |
| `activeTabTextStyle`    | `TextStyle`               | -            | Custom style for active tab text                         |
| `inactiveTabTextStyle`  | `TextStyle`               | -            | Custom style for inactive tab text                       |
| `showIndicator`         | `boolean`                 | `true`       | Show bottom indicator                                    |
| `indicatorStyle`        | `ViewStyle`               | -            | Custom style for indicator                               |
| `scrollable`            | `boolean`                 | `false`      | Enable horizontal scrolling for tab bar                  |
| `autoScrollToCenter`    | `boolean`                 | `true`       | Auto-scroll active tab to center when scrollable is true |
| `tabBarBackgroundColor` | `string`                  | -            | Background color for tab bar                             |
| `contentContainerStyle` | `ViewStyle`               | -            | Custom style for container                               |

### TabItem

| Prop        | Type              | Description                        |
| ----------- | ----------------- | ---------------------------------- |
| `key`       | `string`          | Unique identifier for the tab      |
| `title`     | `string`          | Tab title displayed in the tab bar |
| `component` | `React.ReactNode` | Content to render for this tab     |

## Advanced Usage

### Scrollable Tab Bar with Auto-Center

```tsx
<Tabs
  tabs={tabs}
  scrollable={true}
  autoScrollToCenter={true}
  tabBarStyle={{ paddingHorizontal: 16 }}
/>
```

The `autoScrollToCenter` feature automatically scrolls the active tab to the center of the screen when:

- `scrollable` is enabled
- A tab is selected (either by tap or swipe)
- The tab bar has more tabs than can fit on screen

This provides a better user experience by keeping the active tab visible and centered.

### Custom Styling

```tsx
<Tabs
  tabs={tabs}
  activeTabStyle={{
    backgroundColor: "#f0f0f0",
    borderRadius: 8,
  }}
  activeTabTextStyle={{
    color: "#007AFF",
    fontWeight: "bold",
  }}
  tabBarStyle={{
    paddingVertical: 8,
    backgroundColor: "#ffffff",
  }}
/>
```

### Custom Theme Integration

The component automatically uses the app's theme system. You can override colors:

```tsx
<Tabs
  tabs={tabs}
  tabBarBackgroundColor="#f8f9fa"
  activeTabTextStyle={{ color: "#007AFF" }}
  inactiveTabTextStyle={{ color: "#6c757d" }}
/>
```

## Performance

- Uses `react-native-pager-view` for native performance
- Implements `useCallback` for event handlers
- Optimized re-renders with proper state management
- Smooth page transitions with native animations

## Accessibility

- Proper touch targets for tab buttons
- Clear visual indicators for active state
- Support for screen readers (can be enhanced further)
