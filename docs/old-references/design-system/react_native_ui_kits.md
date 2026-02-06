# React Native UI Kit Options for Kiwari POS

Based on the **Kiwari POS Design System** (Vibrant Brand, specific color tokens like `#0c7721` Green and `#ffd500` Yellow, and distinct Light/Dark modes), here are the 3 best UI Kit options for React Native.

Selection criteria: Ability to handle **custom theming** (crucial for specific brand colors) and **flexibility** (to match custom card shapes and borders).

## 1. Tamagui (Recommended)

Tamagui is currently the most powerful option for building strict design systems. It allows you to define your tokens (colors, spacing, radius) exactly as they appear in your `README.md` and enforces them with a compiler for high performance.

### Pros
*   **Perfect for Design Systems:** You can define your `primary-green`, `primary-yellow`, and `surface-grey` as first-class tokens.
*   **Performance:** Uses an optimizing compiler to flatten styles, making it faster than most other kits.
*   **Theming:** Best-in-class support for Light/Dark modes (Vibrant Light/Dark) with automatic theme flipping.
*   **Headless-first:** Gives you the primitives (Stack, Text, Button) to build your exact "Card" design without fighting pre-made styles.

### Cons
*   **Steep Learning Curve:** Configuration can be complex initially.
*   **Setup:** Requires babel plugin setup for full performance (though works without it).

## 2. Gluestack UI (formerly NativeBase)

Gluestack is a "utility-first" library that is very accessible and easy to customize. It uses a configuration file where you can paste your Kiwari color palette and border radii, and they become available as props (e.g., `bg="$primaryGreen"`).

### Pros
*   **Easy Customization:** You can edit the `gluestack-ui.config.ts` to match your `kiwari-spec` perfectly.
*   **Accessibility:** Built-in accessibility support (ARIA roles) which is great for POS systems.
*   **Unstyled Option:** You can use the unstyled version to build your custom "Menu Item Card" without overriding default "Material" or "iOS" looks.

### Cons
*   **Boilerplate:** The config file can get large.
*   **Performance:** Slightly heavier than Tamagui or bare styles.

## 3. React Native Paper (v5)

If you prefer a "batteries-included" approach with ready-made components (like AppBars, FABs, Modals), this is the standard choice. It follows Material Design but v5 has a robust theming system.

### Pros
*   **Speed:** Fastest way to get a working UI. You don't need to build a "Button" component; you just import it.
*   **Community:** Huge community and support.
*   **Theming:** You can override the default colors with your `#0c7721` and `#ffd500`.

### Cons
*   **"Material" Look:** It will look like a Google app by default. You will have to fight the library (override shadows, ripples, and roundness) to match the specific "Kiwari" aesthetic (e.g., your custom yellow borders).
*   **Less Flexible:** Harder to create completely custom components like your "Horizontal Menu Card" compared to Tamagui or Gluestack.

## Summary Recommendation

For **Kiwari POS**, I recommend **Tamagui** or **Gluestack UI**.

*   Choose **Tamagui** if you want maximum performance and a strict adherence to your design tokens.
*   Choose **Gluestack UI** if you want an easier start and utility-style props (like Tailwind).
