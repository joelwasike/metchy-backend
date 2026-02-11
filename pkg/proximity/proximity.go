package proximity

// Label returns a privacy-safe proximity label based on progress (0-100).
// Progress = (1 - distance/maxRadius) * 100; 100 = very close, 0 = at max radius.
func Label(progressPct float64) string {
	switch {
	case progressPct >= 75:
		return "Very Close"
	case progressPct >= 50:
		return "Nearby"
	case progressPct >= 25:
		return "Within Area"
	case progressPct > 0:
		return "Far (within range)"
	default:
		return ""
	}
}

// Progress computes proximity progress: (1 - distance/maxRadius) * 100.
// If distance > maxRadius, returns 0.
func Progress(distanceKm, maxRadiusKm float64) float64 {
	if maxRadiusKm <= 0 {
		return 0
	}
	if distanceKm >= maxRadiusKm {
		return 0
	}
	p := (1 - distanceKm/maxRadiusKm) * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
