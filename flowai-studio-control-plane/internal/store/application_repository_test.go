package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/applications"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeApplicationQueries struct {
	application controlstore.ControlApplication
	list        []controlstore.ListApplicationsForUserRow
	err         error
	create      controlstore.CreateApplicationParams
	update      controlstore.UpdateApplicationParams
	status      controlstore.SetApplicationStatusParams
	deleted     pgtype.UUID
}

func (queries *fakeApplicationQueries) CreateApplication(_ context.Context, params controlstore.CreateApplicationParams) (controlstore.ControlApplication, error) {
	queries.create = params
	return queries.application, queries.err
}

func (queries *fakeApplicationQueries) ListApplicationsForUser(context.Context, pgtype.UUID) ([]controlstore.ListApplicationsForUserRow, error) {
	return queries.list, queries.err
}

func (queries *fakeApplicationQueries) GetApplicationByID(context.Context, pgtype.UUID) (controlstore.ControlApplication, error) {
	return queries.application, queries.err
}

func (queries *fakeApplicationQueries) UpdateApplication(_ context.Context, params controlstore.UpdateApplicationParams) (controlstore.ControlApplication, error) {
	queries.update = params
	return queries.application, queries.err
}

func (queries *fakeApplicationQueries) DeleteApplication(_ context.Context, id pgtype.UUID) (pgtype.UUID, error) {
	queries.deleted = id
	return id, queries.err
}

func (queries *fakeApplicationQueries) SetApplicationStatus(_ context.Context, params controlstore.SetApplicationStatusParams) (controlstore.ControlApplication, error) {
	queries.status = params
	return queries.application, queries.err
}

func TestApplicationRepositoryConvertsCreateGetAndListRows(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	databaseApplication := sampleDatabaseApplication(t, now)
	queries := &fakeApplicationQueries{
		application: databaseApplication,
		list: []controlstore.ListApplicationsForUserRow{
			applicationListRow(databaseApplication, "owner"),
			applicationListRow(databaseApplication, []byte("full_access")),
		},
	}
	repository := NewApplicationRepository(queries)
	description := "description"
	icon := "icon"

	created, err := repository.CreateApplication(context.Background(), "e9f6332d-da39-44b2-917c-da5ff30aca9d", applications.CreateInput{
		Name:        "App",
		Description: &description,
		Icon:        &icon,
		Status:      applications.StatusDraft,
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if created.ID != "7a611d9a-b555-4469-a289-f1672daefce3" || created.OwnerID != "e9f6332d-da39-44b2-917c-da5ff30aca9d" {
		t.Fatalf("created = %#v", created)
	}
	if !queries.create.Description.Valid || !queries.create.Icon.Valid || queries.create.Status != "draft" {
		t.Fatalf("create params = %#v", queries.create)
	}

	listed, err := repository.ListApplications(context.Background(), "e9f6332d-da39-44b2-917c-da5ff30aca9d")
	if err != nil || len(listed) != 2 || listed[0].AccessType != "owner" || listed[1].AccessType != "full_access" {
		t.Fatalf("listed = %#v, error = %v", listed, err)
	}

	got, err := repository.GetApplication(context.Background(), created.ID)
	if err != nil || got.CreatedAt != now || got.Name != "App" {
		t.Fatalf("got = %#v, error = %v", got, err)
	}
}

func TestApplicationRepositoryPreservesPatchNullAndStatusFlags(t *testing.T) {
	queries := &fakeApplicationQueries{application: sampleDatabaseApplication(t, time.Now().UTC())}
	repository := NewApplicationRepository(queries)
	name := "Updated"
	status := applications.StatusPublished

	_, err := repository.UpdateApplication(context.Background(), "7a611d9a-b555-4469-a289-f1672daefce3", applications.UpdateInput{
		Name:           &name,
		DescriptionSet: true,
		Description:    nil,
		IconSet:        true,
		Icon:           nil,
		Status:         &status,
	})
	if err != nil {
		t.Fatalf("UpdateApplication() error = %v", err)
	}
	params := queries.update
	if !params.SetName || params.Name != name || !params.SetDescription || params.Description.Valid || !params.SetIcon || params.Icon.Valid || !params.SetStatus || params.Status != "published" {
		t.Fatalf("update params = %#v", params)
	}

	_, err = repository.SetApplicationStatus(context.Background(), "7a611d9a-b555-4469-a289-f1672daefce3", applications.StatusArchived)
	if err != nil || queries.status.Status != "archived" {
		t.Fatalf("status params = %#v, error = %v", queries.status, err)
	}
}

func TestApplicationRepositoryMapsMissingRowsAndInvalidIDs(t *testing.T) {
	repository := NewApplicationRepository(&fakeApplicationQueries{err: pgx.ErrNoRows})
	_, err := repository.GetApplication(context.Background(), "7a611d9a-b555-4469-a289-f1672daefce3")
	if !errors.Is(err, applications.ErrApplicationNotFound) {
		t.Fatalf("error = %v", err)
	}

	_, err = repository.GetApplication(context.Background(), "not-a-uuid")
	if !errors.Is(err, applications.ErrApplicationNotFound) {
		t.Fatalf("invalid ID error = %v", err)
	}
}

func sampleDatabaseApplication(t *testing.T, now time.Time) controlstore.ControlApplication {
	t.Helper()
	return controlstore.ControlApplication{
		ID:          mustDatabaseUUID(t, "7a611d9a-b555-4469-a289-f1672daefce3"),
		Name:        "App",
		Description: pgtype.Text{String: "description", Valid: true},
		Icon:        pgtype.Text{String: "icon", Valid: true},
		Status:      "draft",
		OwnerID:     mustDatabaseUUID(t, "e9f6332d-da39-44b2-917c-da5ff30aca9d"),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}
}

func applicationListRow(application controlstore.ControlApplication, access interface{}) controlstore.ListApplicationsForUserRow {
	return controlstore.ListApplicationsForUserRow{
		ID:          application.ID,
		Name:        application.Name,
		Description: application.Description,
		Icon:        application.Icon,
		Status:      application.Status,
		ShareLink:   application.ShareLink,
		OwnerID:     application.OwnerID,
		CreatedAt:   application.CreatedAt,
		UpdatedAt:   application.UpdatedAt,
		AccessType:  access,
	}
}
