//go:build smoke

package app

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSmokeReadOnly(t *testing.T) {
	if os.Getenv("INTERVALS_ACCESS_TOKEN") == "" && os.Getenv("INTERVALS_API_KEY") == "" {
		t.Skip("set INTERVALS_ACCESS_TOKEN or INTERVALS_API_KEY")
	}
	activityID := os.Getenv("INTERVALS_SMOKE_ACTIVITY_ID")
	wellnessDate := os.Getenv("INTERVALS_SMOKE_WELLNESS_DATE")
	if activityID == "" || wellnessDate == "" {
		t.Skip("set INTERVALS_SMOKE_ACTIVITY_ID and INTERVALS_SMOKE_WELLNESS_DATE")
	}

	client, err := NewClient(Config{
		BaseURL:   "https://intervals.icu",
		Timeout:   30 * time.Second,
		UserAgent: "intervals/smoke",
		Stderr:    os.Stderr,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()
	if _, err := client.AuthStatus(ctx); err != nil {
		t.Fatalf("AuthStatus: %v", err)
	}
	if _, err := client.WhoAmI(ctx); err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if _, err := client.ActivitiesList(ctx, ActivityListOptions{Oldest: wellnessDate}); err != nil {
		t.Fatalf("ActivitiesList: %v", err)
	}
	if _, err := client.ActivityGet(ctx, activityID); err != nil {
		t.Fatalf("ActivityGet: %v", err)
	}
	if _, err := client.WellnessGet(ctx, wellnessDate); err != nil {
		t.Fatalf("WellnessGet: %v", err)
	}
}
