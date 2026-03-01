package domain

const (
	RoleClient    = "CLIENT"
	RoleCompanion = "COMPANION"
	RoleAdmin     = "ADMIN"
)

const (
	PresenceOnline   = "ONLINE"
	PresenceOffline  = "OFFLINE"
	PresenceBusy     = "BUSY"
	PresenceInSession = "IN_SESSION"
)

const (
	InteractionTypeChat = "CHAT"
	InteractionTypeVideo = "VIDEO"
	InteractionTypeBooking = "BOOKING"
)

const (
	RequestStatusPending    = "PENDING"
	RequestStatusPendingKYC = "PENDING_KYC" // payment done but client KYC not complete; request not sent to companion yet
	RequestStatusAccepted   = "ACCEPTED"
	RequestStatusRejected   = "REJECTED"
	RequestStatusExpired    = "EXPIRED"
)

const (
	MediaTypeImage = "IMAGE"
	MediaTypeVideo = "VIDEO"
)

const (
	MediaVisibilityPublic  = "PUBLIC"
	MediaVisibilityPrivate = "PRIVATE"
)

// Search radius options in km
var SearchRadiusKm = []float64{1, 3, 5, 10, 25}

// Wallet transaction types
const (
	WalletTxTypeEarning            = "EARNING"
	WalletTxTypeWithdrawal         = "WITHDRAWAL"
	WalletTxTypeBoostPayment       = "BOOST_PAYMENT"
	WalletTxTypeReferralCommission = "REFERRAL_COMMISSION"
	WalletTxTypeReferralBonus      = "REFERRAL_BONUS"
	WalletTxTypeRefund             = "REFUND"
	WalletTxTypePlatformFee        = "PLATFORM_FEE"
)

// Platform fee: markup added to companion's base price (our profit)
const (
	PlatformFeeSmallCents = 20000  // KES 200 for base price <= KES 2000
	PlatformFeeLargeCents = 50000  // KES 500 for base price > KES 2000
	PlatformFeeThreshold  = 200000 // KES 2000 in cents
	CompanionPayoutRate   = 0.95   // Companion can withdraw 95% of their set price
)

// PlatformFee returns the markup in cents for a given companion base price.
func PlatformFee(baseCents int64) int64 {
	if baseCents <= PlatformFeeThreshold {
		return PlatformFeeSmallCents
	}
	return PlatformFeeLargeCents
}

// ClientPrice returns the price the client sees (base + markup).
func ClientPrice(baseCents int64) int64 {
	return baseCents + PlatformFee(baseCents)
}

// CompanionBaseCents extracts the companion's base price from the client-facing price.
func CompanionBaseCents(clientCents int64) int64 {
	if clientCents <= PlatformFeeThreshold+PlatformFeeSmallCents {
		return clientCents - PlatformFeeSmallCents
	}
	return clientCents - PlatformFeeLargeCents
}

// CompanionPayout returns the amount the companion can withdraw (95% of base).
func CompanionPayout(baseCents int64) int64 {
	return int64(float64(baseCents) * CompanionPayoutRate)
}

// Referral commission rate (5%) and max qualifying transactions per referral
const (
	ReferralCommissionRate     = 0.05
	ReferralMaxTransactions    = 2
)

// System setting keys (admin-configurable via dashboard)
const (
	SettingReferralBonusReferrer = "referral_bonus_referrer_cents" // KES cents credited to referrer
	SettingReferralBonusReferred = "referral_bonus_referred_cents" // KES cents credited to new companion
	SettingReferralCommissionRate = "referral_commission_rate"
	SettingReferralMaxTx          = "referral_max_transactions"
)
