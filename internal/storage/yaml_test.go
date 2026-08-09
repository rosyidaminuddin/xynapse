package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"xynapse/internal/models"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	return NewStorage(t.TempDir())
}

func TestWriteAndReadTicket(t *testing.T) {
	s := newTestStorage(t)
	ticket := &models.Ticket{
		ID:              "100",
		Key:             "PROJ-1",
		Project:         "PROJ",
		Type:            "Story",
		Summary:         "Summary",
		Status:          "Open",
		Assignee:        "Adin",
		Description:     `{"type":"doc"}`,
		DescriptionText: "plain",
		FetchedAt:       time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC),
	}

	if err := s.WriteTicket(ticket); err != nil {
		t.Fatalf("WriteTicket: %v", err)
	}

	got, err := s.ReadTicket("PROJ", "1")
	if err != nil {
		t.Fatalf("ReadTicket: %v", err)
	}

	if got.Key != ticket.Key {
		t.Errorf("Key = %q, want %q", got.Key, ticket.Key)
	}
	if got.DescriptionText != ticket.DescriptionText {
		t.Errorf("DescriptionText = %q, want %q", got.DescriptionText, ticket.DescriptionText)
	}
	if got.Description != ticket.Description {
		t.Errorf("Description = %q, want %q", got.Description, ticket.Description)
	}
	if !got.FetchedAt.Equal(ticket.FetchedAt) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, ticket.FetchedAt)
	}
}

func TestReadTicketNotFound(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.ReadTicket("PROJ", "999")
	if err == nil {
		t.Fatal("expected error for missing ticket")
	}
}

func TestSprintManifestRoundTrip(t *testing.T) {
	s := newTestStorage(t)
	project := "PROJ"

	tickets := []*models.Ticket{
		{ID: "1", Key: "PROJ-1", Project: project, Summary: "one", FetchedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "2", Key: "PROJ-2", Project: project, Summary: "two", FetchedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}

	if err := s.WriteSprintManifest(project, 42, "Sprint 42", tickets); err != nil {
		t.Fatalf("WriteSprintManifest: %v", err)
	}

	manifest, err := s.ReadSprintManifest(project)
	if err != nil {
		t.Fatalf("ReadSprintManifest: %v", err)
	}

	if manifest.SprintID != 42 {
		t.Errorf("SprintID = %d, want 42", manifest.SprintID)
	}
	if manifest.SprintName != "Sprint 42" {
		t.Errorf("SprintName = %q, want %q", manifest.SprintName, "Sprint 42")
	}
	if len(manifest.TicketKeys) != 2 {
		t.Errorf("TicketKeys len = %d, want 2", len(manifest.TicketKeys))
	}

	got, err := s.ReadSprintTickets(project)
	if err != nil {
		t.Fatalf("ReadSprintTickets: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ReadSprintTickets len = %d, want 2", len(got))
	}
	if got[0].Key != "PROJ-1" {
		t.Errorf("first ticket key = %q, want PROJ-1", got[0].Key)
	}
}

func TestReadSprintManifestNotFound(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.ReadSprintManifest("PROJ")
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestStorageLayout(t *testing.T) {
	base := t.TempDir()
	s := NewStorage(base)

	ticket := &models.Ticket{
		Key:       "PROJ-1",
		Project:   "PROJ",
		FetchedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.WriteTicket(ticket); err != nil {
		t.Fatalf("WriteTicket: %v", err)
	}

	want := filepath.Join(base, "PROJ", "PROJ-1.yml")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected ticket file at %s: %v", want, err)
	}
}
