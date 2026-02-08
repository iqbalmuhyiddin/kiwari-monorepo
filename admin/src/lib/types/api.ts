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

export interface OrderItem {
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
	items: OrderItem[];
}

export interface OrderListResponse {
	orders: ActiveOrder[];
	limit: number;
	offset: number;
}
