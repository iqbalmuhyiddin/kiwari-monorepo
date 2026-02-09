package enum

// ── Group A: State machines (CHECK constrained in DB) ──

const (
	OrderStatusNew       = "NEW"
	OrderStatusPreparing = "PREPARING"
	OrderStatusReady     = "READY"
	OrderStatusCompleted = "COMPLETED"
	OrderStatusCancelled = "CANCELLED"
)

const (
	OrderItemStatusPending   = "PENDING"
	OrderItemStatusPreparing = "PREPARING"
	OrderItemStatusReady     = "READY"
)

const (
	CateringStatusBooked    = "BOOKED"
	CateringStatusDPPaid    = "DP_PAID"
	CateringStatusSettled   = "SETTLED"
	CateringStatusCancelled = "CANCELLED"
)

const (
	PaymentStatusPending   = "PENDING"
	PaymentStatusCompleted = "COMPLETED"
	PaymentStatusFailed    = "FAILED"
)

// ── Group C: Borderline (CHECK constrained in DB) ──

const (
	UserRoleOwner   = "OWNER"
	UserRoleManager = "MANAGER"
	UserRoleCashier = "CASHIER"
	UserRoleKitchen = "KITCHEN"
)

const (
	OrderTypeDineIn   = "DINE_IN"
	OrderTypeTakeaway = "TAKEAWAY"
	OrderTypeDelivery = "DELIVERY"
	OrderTypeCatering = "CATERING"
)

// ── Group B: Configurable labels (no DB constraint) ──

const (
	StationGrill    = "GRILL"
	StationBeverage = "BEVERAGE"
	StationRice     = "RICE"
	StationDessert  = "DESSERT"
)

const (
	PaymentMethodCash     = "CASH"
	PaymentMethodQRIS     = "QRIS"
	PaymentMethodTransfer = "TRANSFER"
)

const (
	DiscountTypePercentage = "PERCENTAGE"
	DiscountTypeFixed      = "FIXED_AMOUNT"
)
