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

	f := repository.DiscoveryFilters{
		Latitude:      lat,
		Longitude:     lng,
		RadiusKm:      radiusKm,
		Category:      category,
		Services:      services,
		MinAge:        minAge,
		MaxAge:        maxAge,
		MinPrice:      minPrice,
		MaxPrice:      maxPrice,
		OnlineOnly:    onlineOnly,
		BoostedFirst:  boostedFirst,
		SortBy:        sortBy,
		Limit:         limit,
		Offset:        offset,
	}
	results, err := h.repo.DiscoverCompanions(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "discovery failed"})
		return
	}
	if len(results) == 0 {
		results, err = h.repo.DiscoverCompanionsFallback(f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "discovery failed"})
			return
		}
	}
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
