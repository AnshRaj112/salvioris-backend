package services

import "testing"

func TestValidateAppointmentType(t *testing.T) {
	valid := []string{"online", "in_person", "walk_in", "emergency"}
	for _, v := range valid {
		if !ValidateAppointmentType(v) {
			t.Fatalf("%s should be valid", v)
		}
	}
	if ValidateAppointmentType("invalid") {
		t.Fatal("invalid should fail")
	}
}

func TestDefaultDuration(t *testing.T) {
	if DefaultDuration(0) != DefaultDuration(60) {
		t.Fatal("zero should default to 60 min")
	}
	if DefaultDuration(30) != 30*60*1e9 {
		t.Fatalf("unexpected duration %v", DefaultDuration(30))
	}
}
