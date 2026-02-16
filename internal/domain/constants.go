package domain

const (
	RoleClient   = "CLIENT"
	RoleCompanion = "COMPANION"
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
