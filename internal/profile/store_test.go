package profile

import (
	"fmt"
	"sync"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/config"
)

func TestNormalizeName(t *testing.T) {
	if _, err := NormalizeName("   "); err == nil {
		t.Fatal("expected error for empty profile name")
	}
	name, err := NormalizeName(" work ")
	if err != nil {
		t.Fatalf("NormalizeName failed: %v", err)
	}
	if name != "work" {
		t.Fatalf("unexpected normalized name: %q", name)
	}

	invalid := []string{"work profile", "../work", "work/profile", "work.profile"}
	for _, value := range invalid {
		if _, err := NormalizeName(value); err == nil {
			t.Fatalf("expected invalid profile name error for %q", value)
		}
	}

	allowed := []string{"work", "work_1", "Work-Prod"}
	for _, value := range allowed {
		got, err := NormalizeName(value)
		if err != nil {
			t.Fatalf("expected %q to be valid, got %v", value, err)
		}
		if got != value {
			t.Fatalf("expected normalized name %q, got %q", value, got)
		}
	}
}

func TestValidateRecord(t *testing.T) {
	record := config.ProfileRecord{
		Name:     "work",
		Audience: AudienceEnterprise,
		ClientID: "abc",
		AuthMode: AuthModeDelegated,
	}
	if err := ValidateRecord(record); err != nil {
		t.Fatalf("ValidateRecord failed: %v", err)
	}

	record.Audience = "invalid"
	if err := ValidateRecord(record); err == nil {
		t.Fatal("expected invalid audience error")
	}
}

func TestConcurrentUpsertsPreserveProfiles(t *testing.T) {
	t.Setenv("M365CLI_CONFIG_DIR", t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := NewStore().Upsert(config.ProfileRecord{Name: fmt.Sprintf("p%d", i), Audience: AudienceConsumer, ClientID: "client"}, true)
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	records, err := NewStore().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 40 {
		t.Fatalf("retained %d of 40 successful profiles", len(records))
	}
	active := 0
	for _, r := range records {
		if r.Active {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active profiles: %d", active)
	}
}
