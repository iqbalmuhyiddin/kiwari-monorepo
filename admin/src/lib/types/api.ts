/**
 * Shared API types used across server and client code.
 *
 * Purpose: Eliminate duplication of LoginResponse and User types
 * across hooks.server.ts, login/+page.server.ts, auth store, and components.
 */

export type UserRole = 'OWNER' | 'ADMIN' | 'CASHIER' | 'KITCHEN';

export interface SessionUser {
	id: string;
	outlet_id: string;
	full_name: string;
	email: string;
	role: UserRole;
}

export interface LoginResponse {
	access_token: string;
	refresh_token: string;
	user: {
		id: string;
		outlet_id: string;
		full_name: string;
		email: string;
		role: string;
	};
}

// ── Dashboard / Reports types ────────────────────

export interface DailySales {
	date: string;
	order_count: number;
	total_revenue: string;
	total_discount: string;
	net_revenue: string;
}

export interface HourlySales {
	hour: number;
	order_count: number;
	total_revenue: string;
}

export interface PaymentSummary {
	payment_method: string;
	transaction_count: number;
	total_amount: string;
}

export interface ActiveOrderItem {
	id: string;
	product_name: string;
	quantity: number;
	unit_price: string;
	subtotal: string;
}

export interface ActiveOrder {
	id: string;
	order_number: string;
	order_type: string;
	status: string;
	total_amount: string;
	created_at: string;
	items: ActiveOrderItem[];
}

export interface OrderListResponse {
	orders: ActiveOrder[];
	limit: number;
	offset: number;
}

// ── Menu Management types ────────────────────

export type Station = 'GRILL' | 'BEVERAGE' | 'RICE' | 'DESSERT';

export interface Category {
	id: string;
	outlet_id: string;
	name: string;
	description: string | null;
	sort_order: number;
	is_active: boolean;
	created_at: string;
}

export interface Product {
	id: string;
	outlet_id: string;
	category_id: string;
	name: string;
	description: string | null;
	base_price: string;
	image_url: string | null;
	station: Station | null;
	preparation_time: number | null;
	is_combo: boolean;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface Variant {
	id: string;
	variant_group_id: string;
	name: string;
	price_adjustment: string;
	sort_order: number;
	is_active: boolean;
}

export interface VariantGroup {
	id: string;
	product_id: string;
	name: string;
	is_required: boolean;
	sort_order: number;
	is_active: boolean;
	variants?: Variant[];
}

export interface Modifier {
	id: string;
	modifier_group_id: string;
	name: string;
	price: string;
	sort_order: number;
	is_active: boolean;
}

export interface ModifierGroup {
	id: string;
	product_id: string;
	name: string;
	min_select: number;
	max_select: number | null;
	sort_order: number;
	is_active: boolean;
	modifiers?: Modifier[];
}

export interface ComboItem {
	id: string;
	combo_id: string;
	product_id: string;
	quantity: number;
	sort_order: number;
}

// ── Orders types ────────────────────

export type OrderStatus = 'NEW' | 'PREPARING' | 'READY' | 'COMPLETED' | 'CANCELLED';
export type OrderType = 'DINE_IN' | 'TAKEAWAY' | 'DELIVERY' | 'CATERING';
export type PaymentMethod = 'CASH' | 'QRIS' | 'TRANSFER';
export type KitchenStatus = 'PENDING' | 'PREPARING' | 'READY';

export interface Order {
	id: string;
	outlet_id: string;
	order_number: string;
	customer_id: string | null;
	customer_name?: string;
	customer_phone?: string;
	order_type: OrderType;
	status: OrderStatus;
	table_number: string | null;
	notes: string | null;
	subtotal: string;
	discount_type: 'PERCENTAGE' | 'FIXED_AMOUNT' | null;
	discount_value: string | null;
	discount_amount: string;
	tax_amount: string;
	total_amount: string;
	catering_date: string | null;
	catering_status: 'BOOKED' | 'DP_PAID' | 'SETTLED' | null;
	catering_dp_amount: string | null;
	delivery_platform: 'GOJEK' | 'GRAB' | 'INTERNAL' | null;
	delivery_address: string | null;
	created_by: string;
	created_at: string;
	updated_at: string;
	items?: OrderItem[];
	payments?: Payment[];
}

export interface OrderItem {
	id: string;
	product_id: string;
	product_name?: string;
	variant_id: string | null;
	variant_name?: string;
	quantity: number;
	unit_price: string;
	discount_type: 'PERCENTAGE' | 'FIXED_AMOUNT' | null;
	discount_value: string | null;
	discount_amount: string;
	subtotal: string;
	notes: string | null;
	status: KitchenStatus;
	station: Station | null;
	modifiers?: OrderItemModifier[];
}

export interface OrderItemModifier {
	id: string;
	modifier_id: string;
	modifier_name?: string;
	quantity: number;
	unit_price: string;
}

export interface Payment {
	id: string;
	order_id: string;
	payment_method: PaymentMethod;
	amount: string;
	status: string;
	reference_number: string | null;
	amount_received: string | null;
	change_amount: string | null;
	processed_by: string;
	processed_at: string;
}

export interface FullOrderListResponse {
	orders: Order[];
	limit: number;
	offset: number;
}

// ── Customer CRM types ────────────────────

export interface Customer {
	id: string;
	outlet_id: string;
	name: string;
	phone: string;
	email: string | null;
	notes: string | null;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface CustomerTopItem {
	product_id: string;
	product_name: string;
	total_qty: number;
	total_revenue: string;
}

export interface CustomerStats {
	total_orders: number;
	total_spend: string;
	avg_ticket: string;
	top_items: CustomerTopItem[];
}
