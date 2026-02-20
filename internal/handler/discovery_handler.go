package handler

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"lusty/internal/repository"
	"lusty/pkg/proximity"

	"github.com/gin-gonic/gin"
)

type DiscoveryHandler struct {
	repo *repository.DiscoveryRepository
}

func NewDiscoveryHandler(repo *repository.DiscoveryRepository) *DiscoveryHandler {
	return &DiscoveryHandler{repo: repo}
}

// Discover returns companions for Tinder-style discovery (no exact coordinates).
func (h *DiscoveryHandler) Discover(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.Query("lat"), 64)
	lng, _ := strconv.ParseFloat(c.Query("lng"), 64)
	radiusKm, _ := strconv.ParseFloat(c.DefaultQuery("radius_km", "10"), 64)
	if radiusKm <= 0 || radiusKm > 25 {
		radiusKm = 10
	}
	var minAge, maxAge *int
	var minPrice, maxPrice *int64
	if v := c.Query("min_age"); v != "" {
		a, _ := strconv.Atoi(v)
		minAge = &a
	}
	if v := c.Query("max_age"); v != "" {
		a, _ := strconv.Atoi(v)
		maxAge = &a
	}
	if v := c.Query("min_price"); v != "" {
		p, _ := strconv.ParseInt(v, 10, 64)
		if p >= 0 {
			minPrice = &p
		}
	}
	if v := c.Query("max_price"); v != "" {
		p, _ := strconv.ParseInt(v, 10, 64)
		if p >= 0 {
			maxPrice = &p
		}
	}
	onlineOnly := c.Query("online_only") == "1" || c.Query("online_only") == "true"
	boostedFirst := c.DefaultQuery("boost_first", "true") != "false"
	sortBy := c.DefaultQuery("sort", "distance")
	category := strings.TrimSpace(c.Query("category"))
	var services []string
	if svc := strings.TrimSpace(c.Query("services")); svc != "" {
		for _, s := range strings.Split(svc, ",") {
			if t := strings.TrimSpace(s); t != "" {
				services = append(services, t)
			}
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 50 {
		limit = 50
	}

	// allF fetches ALL matching results without pagination so we can merge and paginate
	// from the combined list (location-based companions first, then no-location ones).
	allF := repository.DiscoveryFilters{
		Latitude:     lat,
		Longitude:    lng,
		RadiusKm:     radiusKm,
		Category:     category,
		Services:     services,
		MinAge:       minAge,
		MaxAge:       maxAge,
		MinPrice:     minPrice,
		MaxPrice:     maxPrice,
		OnlineOnly:   onlineOnly,
		BoostedFirst: boostedFirst,
		SortBy:       sortBy,
		Limit:        9999,
		Offset:       0,
	}

	// Location-based companions (within radius, have visible location)
	locationResults, err := h.repo.DiscoverCompanions(allF)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "discovery failed"})
		return
	}

	// Supplement with companions who haven't shared location (or are outside radius)
	fallbackResults, _ := h.repo.DiscoverCompanionsFallback(allF)

	// Merge: location-based first, fallback deduplicated by companion_id
	seen := make(map[uint]bool, len(locationResults))
	merged := locationResults
	for _, r := range locationResults {
		seen[r.CompanionProfile.ID] = true
	}
	for _, r := range fallbackResults {
		if !seen[r.CompanionProfile.ID] {
			merged = append(merged, r)
			seen[r.CompanionProfile.ID] = true
		}
	}

	// Apply pagination from merged list
	from := offset
	if from >= len(merged) {
		c.JSON(http.StatusOK, gin.H{"results": []gin.H{}})
		return
	}
	to := from + limit
	if to > len(merged) {
		to = len(merged)
	}
	results := merged[from:to]
	out := make([]gin.H, len(results))
	for i, r := range results {
		row := gin.H{
			"companion_id":       r.CompanionProfile.ID,
			"display_name":       r.CompanionProfile.DisplayName,
			"main_image_url":     r.CompanionProfile.MainProfileImageURL,
			"city_or_area":       r.CompanionProfile.CityOrArea,
			"age":                r.Age,
			"is_online":          r.IsOnline,
			"last_seen_at":       r.LastSeenAt,
			"is_boosted":         r.IsBoosted,
			"is_available":       r.IsAvailable,
		}
		if r.DistanceKm >= 0 {
			progress := proximity.Progress(r.DistanceKm, radiusKm)
			row["proximity_progress"] = math.Round(progress*10) / 10
			row["proximity_label"] = proximity.Label(progress)
			row["distance_km"] = math.Round(r.DistanceKm*10) / 10
		}
		out[i] = row
	}
	c.JSON(http.StatusOK, gin.H{"results": out})
}

func roundDistance(km float64) float64 {
	return float64(int(km*10+0.5)) / 10
}
