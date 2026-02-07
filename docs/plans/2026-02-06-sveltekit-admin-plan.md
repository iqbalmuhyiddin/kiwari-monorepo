# SvelteKit Web Admin Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the web admin panel for menu management, order monitoring, CRM, reports, and settings.

**Architecture:** SvelteKit 2 app with Svelte 5, Tailwind CSS, server-side rendering for initial load, client-side navigation after. API calls from `+page.server.ts` to avoid CORS. WebSocket for live order updates.

**Tech Stack:** SvelteKit 2, Svelte 5, Tailwind CSS, TypeScript, pnpm

**Design Doc:** `docs/plans/2026-02-06-pos-system-design.md`

**Parent Plan:** `docs/plans/2026-02-06-backend-plan.md` (Milestones 1-7 complete, API ready)

---

## Milestone 9: SvelteKit Web Admin

> Admin panel for menu management, orders, CRM, reports, and live monitoring.

### Task 9.1: Scaffold SvelteKit Project

**Files:**
- Create: `admin/` (SvelteKit project)

```bash
cd admin
pnpm create svelte@latest . # Choose: Skeleton, TypeScript, ESLint, Prettier
pnpm install
pnpm add tailwindcss @tailwindcss/vite
```

Set up:
- Tailwind CSS with Kiwari design tokens as CSS variables
- API client library (`lib/api/client.ts`)
- Auth store (`lib/stores/auth.ts`)
- Layout with sidebar navigation

Design tokens in `app.css`:
```css
:root {
    --primary-green: #0c7721;
    --primary-yellow: #ffd500;
    --border-yellow: #ffea60;
    --accent-red: #d43b0a;
    --dark-grey: #262626;
    --surface-grey: #3a3838;
    --cream-light: #fffcf2;
}
```

**Commit:** `feat: scaffold SvelteKit admin with Tailwind and design tokens`

---

### Task 9.2: Auth Pages (Login + Protected Layout)

**Files:**
- Create: `admin/src/routes/login/+page.svelte`
- Create: `admin/src/routes/(app)/+layout.svelte`
- Create: `admin/src/lib/api/client.ts`
- Create: `admin/src/lib/stores/auth.ts`

Implement:
- Login page with email/password form (Kiwari brand styling)
- JWT stored in memory (not localStorage for security), refresh token in httpOnly cookie
- Protected layout that redirects to login if no token
- Sidebar navigation with role-based visibility

**Commit:** `feat: add admin login page and protected layout`

---

### Task 9.3: Dashboard Page

**Files:**
- Create: `admin/src/routes/(app)/dashboard/+page.svelte`
- Create: `admin/src/lib/components/StatsCard.svelte`
- Create: `admin/src/lib/components/LiveOrders.svelte`

Implement:
- KPI cards: today's revenue, order count, avg ticket, unique customers
- Hourly sales chart (use Chart.js or lightweight alternative)
- Live active orders panel (WebSocket connection)
- Outlet selector for Owner role

**Commit:** `feat: add admin dashboard with KPIs and live orders`

---

### Task 9.4: Menu Management Pages

**Files:**
- Create: `admin/src/routes/(app)/menu/+page.svelte`
- Create: `admin/src/routes/(app)/menu/[productId]/+page.svelte`
- Create: `admin/src/lib/components/ProductForm.svelte`
- Create: `admin/src/lib/components/VariantGroupEditor.svelte`
- Create: `admin/src/lib/components/ModifierGroupEditor.svelte`

Implement:
- Category tabs with CRUD
- Product list with search/filter
- Product detail/edit form with:
  - Basic info (name, price, category, image upload, station)
  - Variant groups (add/edit/reorder/delete)
  - Modifier groups with min/max (add/edit/reorder/delete)
  - Combo items (if is_combo)
  - Active/inactive toggle
- Drag-and-drop reordering for sort_order

**Commit:** `feat: add menu management with variant and modifier editors`

---

### Task 9.5: Orders Page

**Files:**
- Create: `admin/src/routes/(app)/orders/+page.svelte`
- Create: `admin/src/lib/components/OrderDetail.svelte`
- Create: `admin/src/lib/components/OrderTimeline.svelte`

Implement:
- Order list with filters (status, type, date range)
- Catering tab with upcoming bookings, DP status, remaining balance
- Click row → order detail modal with:
  - Items with variants/modifiers
  - Payment breakdown
  - Customer info
  - Status timeline

**Commit:** `feat: add orders page with detail modal and catering view`

---

### Task 9.6: Customer CRM Page

**Files:**
- Create: `admin/src/routes/(app)/customers/+page.svelte`
- Create: `admin/src/routes/(app)/customers/[id]/+page.svelte`
- Create: `admin/src/lib/components/CustomerStats.svelte`

Implement:
- Customer list with search (phone/name)
- Customer detail page:
  - Contact info
  - Stats cards (total spend, visits, avg ticket)
  - Favorite items (top 5)
  - Order history
  - Catering history

**Commit:** `feat: add customer CRM page with stats and history`

---

### Task 9.7: Reports Page

**Files:**
- Create: `admin/src/routes/(app)/reports/+page.svelte`
- Create: `admin/src/lib/components/SalesChart.svelte`
- Create: `admin/src/lib/components/ProductRanking.svelte`

Implement:
- Date range picker
- Tabs: Penjualan, Produk, Pembayaran, Outlet (owner only)
- Charts for each report type
- CSV export button

**Commit:** `feat: add reports page with charts and CSV export`

---

### Task 9.8: Settings & User Management Pages

**Files:**
- Create: `admin/src/routes/(app)/settings/+page.svelte`
- Create: `admin/src/routes/(app)/users/+page.svelte`
- Create: `admin/src/routes/(app)/outlets/+page.svelte`

Implement:
- Settings: tax rate, receipt info, order number format
- User CRUD with role assignment
- Outlet CRUD (Owner only)

**Commit:** `feat: add settings, user management, and outlet management`

---

## Notes for Implementer

- **SvelteKit:** Use server-side rendering (SSR) for initial load, then client-side navigation. API calls from server-side `+page.server.ts` to avoid CORS in most cases.
- **pnpm** for package management.
- **Brand design tokens:** Primary Green `#0c7721`, Primary Yellow `#ffd500`, Border Yellow `#ffea60`, Accent Red `#d43b0a`, Dark Grey `#262626`. Font: Inter (Roboto fallback).
- **Auth:** JWT in memory (not localStorage), refresh token via httpOnly cookie. Redirect to `/login` on 401.
- **WebSocket:** Connect to `ws/outlets/:oid/orders` for live order updates on dashboard.
