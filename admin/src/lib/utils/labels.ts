/**
 * Shared label and formatting utilities for orders.
 * Centralizes status labels, order type labels, and date formatting
 * to avoid duplication across OrderDetail and Orders page.
 */

export function getStatusLabel(status: string): string {
	const labels: Record<string, string> = {
		NEW: 'Baru',
		PREPARING: 'Diproses',
		READY: 'Siap',
		COMPLETED: 'Selesai',
		CANCELLED: 'Dibatalkan'
	};
	return labels[status] ?? status;
}

export function getOrderTypeLabel(type: string): string {
	const labels: Record<string, string> = {
		DINE_IN: 'Makan di Tempat',
		TAKEAWAY: 'Bawa Pulang',
		DELIVERY: 'Pengiriman',
		CATERING: 'Katering'
	};
	return labels[type] ?? type;
}

export function getCateringStatusLabel(status: string): string {
	const labels: Record<string, string> = {
		BOOKED: 'Dipesan',
		DP_PAID: 'DP Dibayar',
		SETTLED: 'Lunas'
	};
	return labels[status] ?? status;
}

export function formatDateTime(iso: string): string {
	const d = new Date(iso);
	return d.toLocaleString('id-ID', {
		day: 'numeric',
		month: 'long',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		timeZone: 'Asia/Jakarta'
	});
}

export function formatDate(iso: string): string {
	const d = new Date(iso);
	return d.toLocaleDateString('id-ID', {
		day: 'numeric',
		month: 'long',
		year: 'numeric',
		timeZone: 'Asia/Jakarta'
	});
}
