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
	RequestStatusPending  = "PENDING"
	RequestStatusAccepted = "ACCEPTED"
	RequestStatusRejected = "REJECTED"
	RequestStatusExpired  = "EXPIRED"
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
