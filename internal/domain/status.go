package domain

const (
	StatusRegistered  = "registered"
	StatusWaitlisted  = "waitlisted"
	StatusCancelled   = "cancelled"
	StatusTransferred = "transferred"
	StatusAttended    = "attended"
	StatusNoShow      = "no_show"
)

const (
	PaymentUnpaid        = "unpaid"
	PaymentPending       = "pending"
	PaymentPaid          = "paid"
	PaymentRejected      = "rejected"
	PaymentRefundPending = "refund_pending"
	PaymentRefunded      = "refunded"
	PaymentForfeited     = "forfeited"
)

const (
	SessionDraft     = "draft"
	SessionPublished = "published"
	SessionClosed    = "closed"
	SessionCancelled = "cancelled"
)

const (
	LevelBeginner     = "beginner"
	LevelIntermediate = "intermediate"
	LevelAdvanced     = "advanced"
)

var Levels = []string{LevelBeginner, LevelIntermediate, LevelAdvanced}

func ValidLevel(l string) bool {
	for _, v := range Levels {
		if v == l {
			return true
		}
	}
	return false
}

// LevelWeight is the balancing weight used by the rotation: beginner 1, intermediate 2, advanced 3.
func LevelWeight(l string) int {
	switch l {
	case LevelAdvanced:
		return 3
	case LevelIntermediate:
		return 2
	default:
		return 1
	}
}

// HoldsSlot reports whether a registration status occupies one of the session's quota seats.
func HoldsSlot(status string) bool {
	return status == StatusRegistered || status == StatusAttended || status == StatusNoShow
}

// IsActive reports whether a registration still takes part in slot allocation.
func IsActive(status string) bool {
	return status == StatusRegistered || status == StatusWaitlisted
}

// CountsAsIncome reports whether money for a registration was received at some point.
func CountsAsIncome(paymentStatus string) bool {
	switch paymentStatus {
	case PaymentPaid, PaymentForfeited, PaymentRefundPending, PaymentRefunded:
		return true
	}
	return false
}
