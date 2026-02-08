/**
 * Format a number or string as Indonesian Rupiah.
 * e.g., "1250000.00" → "Rp 1.250.000"
 */
export function formatRupiah(amount: string | number): string {
	const num = typeof amount === 'string' ? parseFloat(amount) : amount;
	if (isNaN(num)) return 'Rp 0';
	return (
		'Rp ' +
		num.toLocaleString('id-ID', {
			minimumFractionDigits: 0,
			maximumFractionDigits: 0
		})
	);
}
