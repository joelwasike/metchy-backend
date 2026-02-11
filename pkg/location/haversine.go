package location

import "math"

// EarthRadiusKm is the Earth radius in kilometers for Haversine.
const EarthRadiusKm = 6371.0

// HaversineKm returns distance in km between two points (lat/lng in degrees).
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	φ1, φ2 := rad(lat1), rad(lat2)
	Δφ := rad(lat2 - lat1)
	Δλ := rad(lng2 - lng1)
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return EarthRadiusKm * c
}

// FuzzCoordinate adds random offset in meters (converted to degrees approx).
// Used to obfuscate exact location for map display.
func FuzzMeters(meters float64) float64 {
	// ~111km per degree at equator; 1m ≈ 1/111000 degree
	return meters / 111000.0
}
