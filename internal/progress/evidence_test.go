package progress

import "testing"

func TestAttachmentVerificationRequiresCameraImageAndVerifiedLocation(t *testing.T) {
	policy := &GeofenceSnapshot{ID: "g", Version: 1, Latitude: 22.5726, Longitude: 88.3639, RadiusMetres: 100, MaxAccuracyMetres: 20}
	evidence := evaluateEvidence(&ReportedLocation{Latitude: 22.5726, Longitude: 88.3639, AccuracyMetres: 5}, nil, policy)
	if evidence.LocationStatus != "verified" {
		t.Fatalf("evidence=%#v", evidence)
	}
	status, _ := attachmentVerification("image", "camera", evidence.LocationStatus)
	if status != "verified" {
		t.Fatalf("status=%s", status)
	}
	for _, test := range []struct{ kind, source string }{{"image", "upload"}, {"document", "upload"}, {"video", "upload"}} {
		status, _ = attachmentVerification(test.kind, test.source, evidence.LocationStatus)
		if status != "non_verified" {
			t.Fatalf("%s/%s status=%s", test.kind, test.source, status)
		}
	}
}

func TestEvidencePreservesOutsideLocation(t *testing.T) {
	policy := &GeofenceSnapshot{ID: "g", Version: 2, Latitude: 0, Longitude: 0, RadiusMetres: 10, MaxAccuracyMetres: 20}
	location := &ReportedLocation{Latitude: 1, Longitude: 1, AccuracyMetres: 5}
	evidence := evaluateEvidence(location, nil, policy)
	if evidence.LocationStatus != "unverified_outside" || evidence.ReportedLocation != location || evidence.ComputedDistanceMetres == nil {
		t.Fatalf("evidence=%#v", evidence)
	}
}
