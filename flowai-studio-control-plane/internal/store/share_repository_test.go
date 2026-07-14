package store

import (
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/shares"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestShareRecordsDecodeJSONBAndPublicApplication(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	shareID := mustShareUUID(t, "516f794a-eb09-4e7d-bdbd-d4e2bc5f14da")
	applicationID := mustShareUUID(t, "7a611d9a-b555-4469-a289-f1672daefce3")
	row := controlstore.ControlAppShare{
		ID: shareID, ApplicationID: applicationID, ShareLink: "share-00112233445566778899aabbccddeeff",
		IsPublic: true, AccessCount: 4, EmbedConfig: []byte(`{"theme":"dark","showHeader":false}`),
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}

	record, err := shareRecord(row)
	if err != nil || record.ID != "516f794a-eb09-4e7d-bdbd-d4e2bc5f14da" || record.EmbedConfig == nil ||
		record.EmbedConfig.Theme != shares.ThemeDark || record.EmbedConfig.ShowHeader == nil || *record.EmbedConfig.ShowHeader {
		t.Fatalf("shareRecord() = %#v, error = %v", record, err)
	}

	description := "Shared demo"
	public, err := publicShareRecord(controlstore.GetPublicAppShareByLinkRow{
		ID: row.ID, ApplicationID: row.ApplicationID, ShareLink: row.ShareLink, IsPublic: row.IsPublic,
		AccessCount: row.AccessCount, EmbedConfig: row.EmbedConfig, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: "Demo", Description: pgtype.Text{String: description, Valid: true}, Status: "published",
	})
	if err != nil || public.ID != record.ID || public.ApplicationID != record.ApplicationID || public.Description == nil || *public.Description != description {
		t.Fatalf("publicShareRecord() = %#v, error = %v", public, err)
	}
}

func TestShareRecordRejectsInvalidEmbedJSON(t *testing.T) {
	_, err := shareRecord(controlstore.ControlAppShare{
		ID:            mustShareUUID(t, "516f794a-eb09-4e7d-bdbd-d4e2bc5f14da"),
		ApplicationID: mustShareUUID(t, "7a611d9a-b555-4469-a289-f1672daefce3"),
		EmbedConfig:   []byte(`{"theme":`),
	})
	if err == nil {
		t.Fatal("shareRecord() accepted invalid embed JSON")
	}
}

func mustShareUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	result, err := parseUUID(value)
	if err != nil {
		t.Fatalf("parseUUID(%q): %v", value, err)
	}
	return result
}
