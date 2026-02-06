# Kiwari POS Design System

## Overview
The Kiwari POS design system is built around a **"Vibrant Brand"** aesthetic, emphasizing the brand's core colors (Green & Yellow) to create a lively, energetic, and professional interface.

## 1. Color Palette

### Primary Brand Colors
- **Primary Green**: ![#0c7721](https://placehold.co/15x15/0c7721/0c7721.png) `#0c7721` (Main Action Color, Success States)
- **Primary Yellow**: ![#ffd500](https://placehold.co/15x15/ffd500/ffd500.png) `#ffd500` (Headers, Highlights, Accent Text)
- **Border Yellow**: ![#ffea60](https://placehold.co/15x15/ffea60/ffea60.png) `#ffea60` (Subtle borders for cards in Light Mode)
- **Accent Red**: ![#d43b0a](https://placehold.co/15x15/d43b0a/d43b0a.png) `#d43b0a` (Errors, Destructive Actions)

### Neutrals & Surfaces
| Color Name | Hex | Usage |
| :--- | :--- | :--- |
| **Dark Grey** | ![#262626](https://placehold.co/15x15/262626/262626.png) `#262626` | Primary Text (Light Mode), Background (Dark Mode) |
| **Surface Grey** | ![#3a3838](https://placehold.co/15x15/3a3838/3a3838.png) `#3a3838` | Card Background (Dark Mode) |
| **Cream Light** | ![#fffcf2](https://placehold.co/15x15/fffcf2/fffcf2.png) `#fffcf2` | Secondary Backgrounds (Light Mode) |
| **White** | ![#ffffff](https://placehold.co/15x15/ffffff/ffffff.png) `#ffffff` | Main Background (Light Mode) |

## 2. Typography
**Font Family**: Inter (or Roboto as fallback)

- **Headings**: Bold / ExtraBold (Weights 700-800)
- **Body**: Regular / Medium (Weights 400-500)
- **Buttons**: Bold (Weight 700)

## 3. Theme Definitions

### Light Mode (Vibrant Light)
*The default, energetic view.*
- **Background**: White `#ffffff`
- **Header**: Primary Yellow `#ffd500` with Dark Text `#262626`
- **Cards**: White with **Neutral Border `#e0e0e0`** and subtle shadow.
    - **Active State**: **Border Yellow `#ffea60`** (2px solid) + Light Yellow Background `#fffdf5`.
- **Primary Text**: Dark Grey `#262626`
- **Primary Button**: Primary Green `#0c7721` (White Text) or Dark Grey `#262626` (Yellow Text)

### Dark Mode (Vibrant Dark)
*Sleek, high-contrast view for low-light environments.*
- **Background**: Dark Grey `#262626`
- **Header**: Dark Grey `#333333` with Yellow Bottom Border
- **Cards**: Surface Grey `#404040` (Lighter than background for separation).
    - **Price**: Monospace font, Bold, Primary Yellow `#ffd500`.
- **Primary Text**: White `#ffffff`
- **Accent Text**: Primary Yellow `#ffd500`
- **Primary Button**: Primary Yellow `#ffd500` (Dark Text) for high visibility

## 4. Components

### Buttons
- **Shape**: Rounded corners (12px - 16px)
- **Shadow**: Subtle drop shadow for primary actions

### Cards (Menu Items)
- **Shape**: Rounded corners (20px)
- **Layout**: Horizontal list item with image on left
- **Action**: "Add" button is circular and prominent
