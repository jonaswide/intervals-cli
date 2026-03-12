package gen

import (
	"encoding/json"
	"testing"
)

func TestGeneratedModelMarshalling(t *testing.T) {
	uid := "uid-1"
	name := "Workout Day"
	restingHR := int32(48)
	id := "2026-03-12"
	activityID := "a1"
	intervalID := int32(1)

	cases := []any{
		EventEx{Uid: &uid, Name: &name},
		WorkoutEx{Name: &name},
		Wellness{Id: &id, RestingHR: &restingHR},
		UploadResponse{IcuAthleteId: &id, Activities: &[]ActivityId{{Id: &activityID}}},
		ActivityWithIntervals{Id: &activityID, IcuIntervals: &[]Interval{{Id: &intervalID}}},
	}
	for _, tc := range cases {
		data, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("marshal %T: %v", tc, err)
		}
		if len(data) == 0 {
			t.Fatalf("marshal %T produced empty output", tc)
		}
	}
}
