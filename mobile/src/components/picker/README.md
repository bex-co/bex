# Picker Component

A customizable picker component built with react-native-reanimated2 and native ScrollView that displays a modal from the bottom with a wheel picker interface.

## Features

- ✅ Modal slides up from bottom with smooth animation
- ✅ Shows maximum 5 items at a time (wheel effect)
- ✅ Uses native ScrollView for smooth scrolling
- ✅ Snap-to-item functionality
- ✅ Visual selection indicator
- ✅ Fade gradients for better UX
- ✅ Theme-aware styling
- ✅ TypeScript support

## Props

```typescript
type PickerProps = {
  visible: boolean; // Controls modal visibility
  items: { label: string; value: string }[]; // Array of items to display
  onSelect: (item: { label: string; value: string }) => void; // Callback when item is selected
  onCancel: () => void; // Callback when picker is cancelled
  selectedValue?: string; // Currently selected value (optional)
};
```

## Usage

```tsx
import React, { useState } from "react";
import { View, TouchableOpacity, Text } from "react-native";
import { Picker } from "@/components";

const MyComponent = () => {
  const [visible, setVisible] = useState(false);
  const [selectedItem, setSelectedItem] = useState(null);

  const items = [
    { label: "Option 1", value: "option1" },
    { label: "Option 2", value: "option2" },
    { label: "Option 3", value: "option3" },
    // ... more items
  ];

  const handleSelect = (item) => {
    setSelectedItem(item);
    setVisible(false);
  };

  const handleCancel = () => {
    setVisible(false);
  };

  return (
    <View>
      <TouchableOpacity onPress={() => setVisible(true)}>
        <Text>Open Picker</Text>
      </TouchableOpacity>

      <Picker
        visible={visible}
        items={items}
        onSelect={handleSelect}
        onCancel={handleCancel}
        selectedValue={selectedItem?.value}
      />
    </View>
  );
};
```

## Example

See `example.tsx` for a complete working example.

## Styling

The component automatically adapts to your app's theme using the `useTheme` hook. It supports both light and dark themes with appropriate colors for:

- Background colors
- Text colors
- Selection indicator
- Fade gradients
- Button colors

## Technical Details

- **Animation**: Uses react-native-reanimated2 for smooth 60fps animations
- **ScrollView**: Native ScrollView with snap-to-interval for precise item selection
- **Performance**: Optimized with useCallback and useMemo for smooth scrolling
- **Accessibility**: Proper touch targets and keyboard navigation support
- **Safe Area**: Leaves room for the home indicator on iOS devices

## Customization

You can customize the appearance by modifying the `getStyles` function in the component. Key constants that can be adjusted:

- `ITEM_HEIGHT`: Height of each picker item (default: 50)
- `VISIBLE_ITEMS`: Number of items visible at once (default: 5)
- `WHEEL_HEIGHT`: Total height of the wheel (calculated from above)
