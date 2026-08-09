package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func writePlanWithMtime(t *testing.T, base, key string, modTime time.Time) {
	t.Helper()
	writePlanFile(t, base, key, []byte("# plan"))
	if err := os.Chtimes(filepath.Join(base, "plans", key+".md"), modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestPlanStatus(t *testing.T) {
	base := t.TempDir()
	modTime := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
	writePlanWithMtime(t, base, "PROJ-1", modTime)
	s := storage.NewStorage(base)

	cases := []struct {
		name      string
		key       string
		updatedAt time.Time
		want      string
	}{
		{"no plan", "PROJ-9", modTime, PlanNone},
		{"fresh", "PROJ-1", modTime.Add(-time.Hour), PlanFresh},
		{"stale", "PROJ-1", modTime.Add(time.Hour), PlanStale},
		{"zero updatedAt is fresh", "PROJ-1", time.Time{}, PlanFresh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := planStatus(s, "PROJ", tc.key, tc.updatedAt); got != tc.want {
				t.Errorf("planStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStaleWarning(t *testing.T) {
	base := t.TempDir()
	modTime := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
	writePlanWithMtime(t, base, "PROJ-1", modTime)
	s := storage.NewStorage(base)
	if err := s.WriteTicket(&models.Ticket{
		Key: "PROJ-1", Project: "PROJ",
		FetchedAt: time.Now(), UpdatedAt: modTime.Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	msg, ok := staleWarning(s, "PROJ", "PROJ-1", "PROJ-1")
	if !ok {
		t.Fatal("expected stale warning")
	}
	for _, want := range []string{"stale", "xynapse plan PROJ-1"} {
		if !strings.Contains(msg, want) {
			t.Errorf("warning missing %q: %q", want, msg)
		}
	}
}

func TestStaleWarningSkipsWithoutTicket(t *testing.T) {
	base := t.TempDir()
	modTime := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
	writePlanWithMtime(t, base, "PROJ-1", modTime)
	s := storage.NewStorage(base)

	if msg, ok := staleWarning(s, "PROJ", "PROJ-1", "PROJ-1"); ok {
		t.Errorf("expected no warning without a cached ticket, got %q", msg)
	}
}

func TestFilterUnplanned(t *testing.T) {
	views := []SprintTicket{
		{Ticket: &models.Ticket{Key: "PROJ-1"}, Plan: PlanFresh},
		{Ticket: &models.Ticket{Key: "PROJ-2"}, Plan: PlanStale},
		{Ticket: &models.Ticket{Key: "PROJ-3"}, Plan: PlanNone},
		{Ticket: &models.Ticket{Key: "PROJ-4"}, Plan: PlanNone},
	}
	got := filterUnplanned(views)
	if len(got) != 2 || got[0].Ticket.Key != "PROJ-3" || got[1].Ticket.Key != "PROJ-4" {
		t.Errorf("filterUnplanned = %+v, want PROJ-3, PROJ-4", got)
	}
}

func TestConfirmStale(t *testing.T) {
	setup := func(t *testing.T) (*storage.Storage, string) {
		base := t.TempDir()
		modTime := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
		writePlanWithMtime(t, base, "PROJ-1", modTime)
		s := storage.NewStorage(base)
		if err := s.WriteTicket(&models.Ticket{
			Key: "PROJ-1", Project: "PROJ",
			FetchedAt: time.Now(), UpdatedAt: modTime.Add(2 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		return s, base
	}

	t.Run("fresh skips prompt", func(t *testing.T) {
		base := t.TempDir()
		modTime := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
		writePlanWithMtime(t, base, "PROJ-1", modTime)
		s := storage.NewStorage(base)
		if err := s.WriteTicket(&models.Ticket{
			Key: "PROJ-1", Project: "PROJ",
			FetchedAt: time.Now(), UpdatedAt: modTime.Add(-2 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		if err := confirmStale(s, "PROJ", "PROJ-1", "PROJ-1", false); err != nil {
			t.Errorf("fresh plan should not prompt: %v", err)
		}
	})

	t.Run("force skips prompt", func(t *testing.T) {
		s, _ := setup(t)
		if err := confirmStale(s, "PROJ", "PROJ-1", "PROJ-1", true); err != nil {
			t.Errorf("force should skip prompt: %v", err)
		}
	})

	t.Run("yes proceeds", func(t *testing.T) {
		s, _ := setup(t)
		withStdin(t, "y\n", func() {
			if err := confirmStale(s, "PROJ", "PROJ-1", "PROJ-1", false); err != nil {
				t.Errorf("expected proceed, got: %v", err)
			}
		})
	})

	t.Run("no cancels", func(t *testing.T) {
		s, _ := setup(t)
		withStdin(t, "n\n", func() {
			err := confirmStale(s, "PROJ", "PROJ-1", "PROJ-1", false)
			if err == nil {
				t.Fatal("expected cancellation error")
			}
			if !strings.Contains(err.Error(), "cancelled") {
				t.Errorf("error should mention cancelled, got %v", err)
			}
		})
	})

	t.Run("eof cancels", func(t *testing.T) {
		s, _ := setup(t)
		withStdin(t, "", func() {
			if err := confirmStale(s, "PROJ", "PROJ-1", "PROJ-1", false); err == nil {
				t.Fatal("expected cancellation error on EOF")
			}
		})
	})
}

func TestShowPlanWarnsOnStale(t *testing.T) {
	base := t.TempDir()
	plan := "# plan"
	writePlanWithMtime(t, base, "PROJ-1", time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC))
	s := storage.NewStorage(base)
	if err := s.WriteTicket(&models.Ticket{
		Key: "PROJ-1", Project: "PROJ",
		FetchedAt: time.Now(), UpdatedAt: time.Date(2026, 8, 9, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(base)

	errOut := captureStderr(t, func() {
		out := captureStdout(t, func() {
			if err := ShowPlan(cfg, "PROJ-1"); err != nil {
				t.Errorf("ShowPlan: %v", err)
			}
		})
		if out != plan {
			t.Errorf("ShowPlan output = %q, want %q", out, plan)
		}
	})
	if !strings.Contains(errOut, "stale") {
		t.Errorf("expected stale warning on stderr, got %q", errOut)
	}
}
