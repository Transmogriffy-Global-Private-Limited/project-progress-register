package progress

import "math"

const earthRadiusMetres = 6371008.8

func evaluateEvidence(location *ReportedLocation, unavailable *string, geofence *GeofenceSnapshot) Evidence {
	if location == nil {
		if unavailable == nil {
			return Evidence{LocationStatus: "not_supplied", LocationReason: "No browser location was supplied."}
		}
		return Evidence{LocationStatus: "unverified_unavailable", LocationReason: "The browser could not supply a location.", LocationUnavailableReason: unavailable}
	}
	evidence := Evidence{ReportedLocation: location}
	if geofence == nil {
		evidence.LocationStatus = "unverified_no_geofence"
		evidence.LocationReason = "A location was recorded, but the project has no current geofence."
		return evidence
	}
	snapshot := *geofence
	evidence.Geofence = &snapshot
	distance := haversineMetres(location.Latitude, location.Longitude, geofence.Latitude, geofence.Longitude)
	evidence.ComputedDistanceMetres = &distance
	if location.AccuracyMetres > geofence.MaxAccuracyMetres {
		evidence.LocationStatus = "unverified_accuracy"
		evidence.LocationReason = "The browser-reported accuracy exceeded the project limit."
		return evidence
	}
	if distance+location.AccuracyMetres > geofence.RadiusMetres {
		evidence.LocationStatus = "unverified_outside"
		evidence.LocationReason = "The reported uncertainty area was not fully inside the project geofence."
		return evidence
	}
	evidence.LocationStatus = "verified"
	evidence.LocationReason = "The server accepted the reported accuracy and geofence calculation."
	return evidence
}

func attachmentVerification(mediaKind, source, locationStatus string) (string, string) {
	if mediaKind != "image" {
		return "non_verified", "Only Chrome camera photographs are eligible for verified status."
	}
	if source != "camera" {
		return "non_verified", "Existing-file images are not verified camera captures."
	}
	if locationStatus != "verified" {
		return "non_verified", "The camera photograph's upload location was not verified: " + locationStatus + "."
	}
	return "verified", "Chrome camera source reported and upload location verified by the server."
}

func haversineMetres(latitudeA, longitudeA, latitudeB, longitudeB float64) float64 {
	toRadians := func(value float64) float64 { return value * math.Pi / 180 }
	latA, latB := toRadians(latitudeA), toRadians(latitudeB)
	deltaLat, deltaLon := toRadians(latitudeB-latitudeA), toRadians(longitudeB-longitudeA)
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) + math.Cos(latA)*math.Cos(latB)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadiusMetres * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
