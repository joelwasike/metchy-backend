package repository

import (
	"lusty/internal/models"
	"lusty/pkg/location"
	"sort"
	"time"

	"gorm.io/gorm"
)

// DiscoveryFilters for Tinder-style search.
type DiscoveryFilters struct {
	Latitude     float64
	Longitude    float64
	RadiusKm     float64
	Category     string   // e.g. "tall", "slim" - filter by categories
	Services     []string // e.g. ["SEX", "MASSAGE"] - filter by interests (comma-separated in profile)
	MinAge       *int
	MaxAge       *int
	MinPrice     *int64
	MaxPrice     *int64
	OnlineOnly   bool
	BoostedFirst bool
	SortBy       string // distance, recently_active, boost
	Limit        int
	Offset       int
}

type DiscoveryResult struct {
	CompanionProfile models.CompanionProfile
	User             models.User
	DistanceKm       float64
	Age              int
	IsOnline         bool
	LastSeenAt       time.Time
	IsBoosted        bool
}

// DiscoveryRepository performs location-based companion discovery.
// Exact coordinates are not returned; distance is computed server-side.
type DiscoveryRepository struct {
	db *gorm.DB
}

func NewDiscoveryRepository(db *gorm.DB) *DiscoveryRepository {
	return &DiscoveryRepository{db: db}
}

// DiscoverCompanions returns companions within radius with filters.
// Uses Haversine in application layer after bounding box pre-filter for performance.
func (r *DiscoveryRepository) DiscoverCompanions(f DiscoveryFilters) ([]DiscoveryResult, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	// Approximate degree delta for radius (km): 1 deg ~ 111km
	delta := f.RadiusKm / 111.0
	latMin, latMax := f.Latitude-delta, f.Latitude+delta
	lngMin, lngMax := f.Longitude-delta, f.Longitude+delta

	query := r.db.Table("companion_profiles cp").
		Select(`
			cp.id as companion_id, cp.user_id, cp.display_name, cp.bio, cp.main_profile_image_url,
			cp.city_or_area, cp.is_active,
			u.date_of_birth,
			ul.latitude, ul.longitude,
			up.is_online, up.last_seen_at,
			cb.id as boost_id
		`).
		Joins("INNER JOIN users u ON u.id = cp.user_id AND u.deleted_at IS NULL").
		Joins("LEFT JOIN user_locations ul ON ul.user_id = u.id AND ul.deleted_at IS NULL").
		Joins("LEFT JOIN user_presence up ON up.user_id = u.id AND up.deleted_at IS NULL").
		Joins("LEFT JOIN companion_boosts cb ON cb.companion_id = cp.id AND cb.is_active = 1 AND cb.end_at > NOW() AND cb.deleted_at IS NULL").
		Where("cp.deleted_at IS NULL AND cp.is_active = ? AND COALESCE(cp.appear_in_search, 1) = 1", true).
		Where("ul.latitude BETWEEN ? AND ? AND ul.longitude BETWEEN ? AND ?", latMin, latMax, lngMin, lngMax).
		Where("ul.is_location_visible = ?", true)

	if f.Category != "" {
		// Match exact category in comma-separated list
		query = query.Where("CONCAT(',', cp.categories, ',') LIKE ?", "%,"+f.Category+",%")
	}
	for _, svc := range f.Services {
		if svc != "" {
			// Match service in interests (comma-separated: SEX,MASSAGE,OTHER etc)
			query = query.Where("CONCAT(',', COALESCE(cp.interests,''), ',') LIKE ?", "%,"+svc+",%")
		}
	}
	if f.OnlineOnly {
		query = query.Where("up.is_online = ?", true)
	}

	// Subquery to compute distance and filter by Haversine in app (or raw SQL with formula)
	var rows []struct {
		CompanionID uint
		UserID      uint
		DisplayName string
		Bio         string
		MainProfileImageURL string
		CityOrArea  string
		IsActive    bool
		Latitude    float64
		Longitude   float64
		IsOnline    bool
		LastSeenAt  *time.Time
		BoostID     *uint
		DateOfBirth *time.Time
	}

	err := query.Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var results []DiscoveryResult
	now := time.Now()
	for _, row := range rows {
		distKm := location.HaversineKm(f.Latitude, f.Longitude, row.Latitude, row.Longitude)
		if distKm > f.RadiusKm {
			continue
		}
		age := 0
		if row.DateOfBirth != nil {
			age = now.Year() - row.DateOfBirth.Year()
			if now.YearDay() < row.DateOfBirth.YearDay() {
				age--
			}
		}
		if f.MinAge != nil && age < *f.MinAge {
			continue
		}
		if f.MaxAge != nil && age > *f.MaxAge {
			continue
		}

		lastSeen := now
		if row.LastSeenAt != nil {
			lastSeen = *row.LastSeenAt
		}
		results = append(results, DiscoveryResult{
			CompanionProfile: models.CompanionProfile{
				ID:                  row.CompanionID,
				UserID:              row.UserID,
				DisplayName:         row.DisplayName,
				Bio:                 row.Bio,
				MainProfileImageURL:  row.MainProfileImageURL,
				CityOrArea:          row.CityOrArea,
				IsActive:            row.IsActive,
			},
			User:       models.User{ID: row.UserID, DateOfBirth: row.DateOfBirth},
			DistanceKm: distKm,
			Age:        age,
			IsOnline:   row.IsOnline,
			LastSeenAt: lastSeen,
			IsBoosted:  row.BoostID != nil,
		})
	}

	// Sort: boost first, then by distance or last_seen
	sortDiscoveryResults(results, f.SortBy, f.BoostedFirst)
	// Paginate
	from := f.Offset
	if from >= len(results) {
		return []DiscoveryResult{}, nil
	}
	to := from + f.Limit
	if to > len(results) {
		to = len(results)
	}
	return results[from:to], nil
}

func sortDiscoveryResults(r []DiscoveryResult, sortBy string, boostedFirst bool) {
	sort.Slice(r, func(i, j int) bool {
		if boostedFirst && r[i].IsBoosted != r[j].IsBoosted {
			return r[i].IsBoosted
		}
		switch sortBy {
		case "recently_active":
			return r[i].LastSeenAt.After(r[j].LastSeenAt)
		case "distance":
			return r[i].DistanceKm < r[j].DistanceKm
		default:
			return r[i].DistanceKm < r[j].DistanceKm
		}
	})
}

// DiscoverCompanionsFallback returns companions with completed profiles (have photos) when
// location-based discovery returns no results. Used for early adopters or when companions
// haven't shared location yet.
func (r *DiscoveryRepository) DiscoverCompanionsFallback(f DiscoveryFilters) ([]DiscoveryResult, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	query := r.db.Table("companion_profiles cp").
		Select(`
			cp.id as companion_id, cp.user_id, cp.display_name, cp.bio, cp.main_profile_image_url,
			cp.city_or_area, cp.is_active,
			u.date_of_birth,
			0.0 as latitude, 0.0 as longitude,
			up.is_online, up.last_seen_at,
			cb.id as boost_id
		`).
		Joins("INNER JOIN users u ON u.id = cp.user_id AND u.deleted_at IS NULL").
		Joins("LEFT JOIN user_presence up ON up.user_id = u.id AND up.deleted_at IS NULL").
		Joins("LEFT JOIN companion_boosts cb ON cb.companion_id = cp.id AND cb.is_active = 1 AND cb.end_at > NOW() AND cb.deleted_at IS NULL").
		Where("cp.deleted_at IS NULL AND cp.is_active = ? AND COALESCE(cp.appear_in_search, 1) = 1", true).
		Where("(cp.main_profile_image_url != '' AND cp.main_profile_image_url IS NOT NULL) OR EXISTS (SELECT 1 FROM companion_media cm WHERE cm.companion_id = cp.id AND cm.deleted_at IS NULL)")

	if f.Category != "" {
		query = query.Where("CONCAT(',', cp.categories, ',') LIKE ?", "%,"+f.Category+",%")
	}
	for _, svc := range f.Services {
		if svc != "" {
			query = query.Where("CONCAT(',', COALESCE(cp.interests,''), ',') LIKE ?", "%,"+svc+",%")
		}
	}

	var rows []struct {
		CompanionID       uint
		UserID            uint
		DisplayName       string
		Bio               string
		MainProfileImageURL string
		CityOrArea        string
		IsActive          bool
		Latitude          float64
		Longitude         float64
		IsOnline          bool
		LastSeenAt        *time.Time
		BoostID           *uint
		DateOfBirth       *time.Time
	}

	err := query.Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var results []DiscoveryResult
	now := time.Now()
	for _, row := range rows {
		age := 0
		if row.DateOfBirth != nil {
			age = now.Year() - row.DateOfBirth.Year()
			if now.YearDay() < row.DateOfBirth.YearDay() {
				age--
			}
		}
		if f.MinAge != nil && age < *f.MinAge {
			continue
		}
		if f.MaxAge != nil && age > *f.MaxAge {
			continue
		}
		lastSeen := now
		if row.LastSeenAt != nil {
			lastSeen = *row.LastSeenAt
		}
		results = append(results, DiscoveryResult{
			CompanionProfile: models.CompanionProfile{
				ID:                 row.CompanionID,
				UserID:             row.UserID,
				DisplayName:        row.DisplayName,
				Bio:                row.Bio,
				MainProfileImageURL: row.MainProfileImageURL,
				CityOrArea:         row.CityOrArea,
				IsActive:           row.IsActive,
			},
			User:       models.User{ID: row.UserID, DateOfBirth: row.DateOfBirth},
			DistanceKm: -1, // Unknown - companion has no location
			Age:        age,
			IsOnline:   row.IsOnline,
			LastSeenAt: lastSeen,
			IsBoosted:  row.BoostID != nil,
		})
	}

	sortDiscoveryResults(results, f.SortBy, f.BoostedFirst)
	from := f.Offset
	if from >= len(results) {
		return []DiscoveryResult{}, nil
	}
	to := from + f.Limit
	if to > len(results) {
		to = len(results)
	}
	return results[from:to], nil
}
