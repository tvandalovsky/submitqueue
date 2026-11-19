// Copyright (c) 2025 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/submitqueue/stovepipe/entity"
	"github.com/uber/submitqueue/stovepipe/extension/storage"
)

const (
	testFactQueue = "monorepo/main"
	testFactURI   = "git://remote/monorepo/main/deadbeef"
)

func setupValidationFactStoreTest(t *testing.T) (*sql.DB, sqlmock.Sqlmock, storage.ValidationFactStore) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := NewValidationFactStore(db, testMetrics(), testFactQueue)

	return db, mock, store
}

func TestValidationFactStore_Create(t *testing.T) {
	fact := entity.ValidationFact{
		URI:         testFactURI,
		Project:     "project-a",
		ProjectType: "service",
		Degree:      entity.DegreeGreen,
		RequestID:   "1",
		CreatedAt:   1735689600000,
	}

	tests := []struct {
		name      string
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantErrIs error
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO validation_fact").
					WithArgs(testFactQueue, fact.URI, fact.Project, fact.ProjectType, fact.Degree, fact.RequestID, fact.CreatedAt).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "duplicate identity returns ErrAlreadyExists",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO validation_fact").
					WithArgs(testFactQueue, fact.URI, fact.Project, fact.ProjectType, fact.Degree, fact.RequestID, fact.CreatedAt).
					WillReturnError(&mysql.MySQLError{Number: mysqlErrDuplicateEntry})
			},
			wantErr:   true,
			wantErrIs: storage.ErrAlreadyExists,
		},
		{
			name: "other exec error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO validation_fact").
					WithArgs(testFactQueue, fact.URI, fact.Project, fact.ProjectType, fact.Degree, fact.RequestID, fact.CreatedAt).
					WillReturnError(fmt.Errorf("connection reset"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, store := setupValidationFactStoreTest(t)
			defer db.Close()

			tt.setup(mock)

			err := store.Create(context.Background(), fact)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestValidationFactStore_Get(t *testing.T) {
	want := entity.ValidationFact{
		URI:         testFactURI,
		Project:     "project-a",
		ProjectType: "service",
		Degree:      entity.DegreeBroken,
		RequestID:   "2",
		CreatedAt:   1735689600000,
	}

	tests := []struct {
		name      string
		setup     func(mock sqlmock.Sqlmock)
		want      entity.ValidationFact
		wantErr   bool
		wantErrIs error
	}{
		{
			name: "found",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"uri", "project", "project_type", "degree", "request_id", "created_at"}).
					AddRow(want.URI, want.Project, want.ProjectType, want.Degree, want.RequestID, want.CreatedAt)
				mock.ExpectQuery("SELECT uri, project, project_type, degree, request_id, created_at FROM validation_fact").
					WithArgs(testFactQueue, want.URI, want.Project).
					WillReturnRows(rows)
			},
			want: want,
		},
		{
			name: "not found",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT uri, project, project_type, degree, request_id, created_at FROM validation_fact").
					WithArgs(testFactQueue, want.URI, want.Project).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:   true,
			wantErrIs: storage.ErrNotFound,
		},
		{
			name: "query error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT uri, project, project_type, degree, request_id, created_at FROM validation_fact").
					WithArgs(testFactQueue, want.URI, want.Project).
					WillReturnError(fmt.Errorf("connection reset"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, store := setupValidationFactStoreTest(t)
			defer db.Close()

			tt.setup(mock)

			got, err := store.Get(context.Background(), want.URI, want.Project)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestValidationFactStore_ListProjects(t *testing.T) {
	readErr := fmt.Errorf("connection reset")
	columns := []string{"uri", "project", "project_type", "degree", "request_id", "created_at"}
	want := entity.ValidationFact{URI: testFactURI, Project: "project-b", ProjectType: "future-type", Degree: 0.5, RequestID: "2", CreatedAt: 20}
	tests := []struct {
		name     string
		rows     *sqlmock.Rows
		queryErr error
		wantErr  bool
		want     []entity.ValidationFact
	}{
		{name: "found", rows: sqlmock.NewRows(columns).AddRow(want.URI, want.Project, want.ProjectType, want.Degree, want.RequestID, want.CreatedAt), want: []entity.ValidationFact{want}},
		{name: "empty", rows: sqlmock.NewRows(columns)},
		{name: "query failure", queryErr: readErr, wantErr: true},
		{name: "scan failure", rows: sqlmock.NewRows(columns).AddRow(want.URI, want.Project, want.ProjectType, "invalid", want.RequestID, want.CreatedAt), wantErr: true},
		{name: "iteration failure", rows: sqlmock.NewRows(columns).AddRow(want.URI, want.Project, want.ProjectType, want.Degree, want.RequestID, want.CreatedAt).RowError(0, readErr), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, store := setupValidationFactStoreTest(t)
			defer db.Close()
			query := mock.ExpectQuery(`SELECT uri, project, project_type, degree, request_id, created_at FROM validation_fact\s+WHERE queue = \? AND uri = \? AND project > \?\s+ORDER BY project ASC LIMIT \?`).WithArgs(testFactQueue, testFactURI, "project-a", 3)
			if tt.queryErr != nil {
				query.WillReturnError(tt.queryErr)
			} else {
				query.WillReturnRows(tt.rows)
			}
			got, err := store.ListProjects(context.Background(), testFactURI, "project-a", 3)
			if tt.wantErr {
				require.Error(t, err)
				if tt.queryErr != nil {
					assert.ErrorIs(t, err, tt.queryErr)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestValidationFactStore_ListProjectsRejectsInvalidLimit(t *testing.T) {
	for _, limit := range []int{0, -1} {
		db, mock, store := setupValidationFactStoreTest(t)
		_, err := store.ListProjects(context.Background(), testFactURI, "", limit)
		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}
}

func TestValidationFactStore_CreateBatch(t *testing.T) {
	first := entity.ValidationFact{
		URI: testFactURI, Project: "first", ProjectType: "service",
		Degree: 1, RequestID: "1", CreatedAt: 10,
	}
	second := entity.ValidationFact{
		URI: testFactURI, Project: "second", ProjectType: "library",
		Degree: 0, RequestID: "1", CreatedAt: 20,
	}
	retained := first
	retained.ProjectType = "retained-type"
	retained.Degree = 0.25
	retained.RequestID = "older-request"
	retained.CreatedAt = 5
	writeErr := &mysql.MySQLError{Number: 1406, Message: "Data too long"}
	readErr := errors.New("connection lost")
	tests := []struct {
		name    string
		input   []entity.ValidationFact
		setup   func(sqlmock.Sqlmock)
		want    []entity.ValidationFact
		wantErr error
	}{
		{name: "empty input"},
		{
			name:  "new facts in one insert and one read",
			input: []entity.ValidationFact{first, second},
			setup: func(mock sqlmock.Sqlmock) {
				expectFactBatchInsert(mock, []entity.ValidationFact{first, second}).WillReturnResult(sqlmock.NewResult(0, 2))
				expectFactBatchRead(mock, []entity.ValidationFact{first, second}).
					WillReturnRows(factBatchRows([]entity.ValidationFact{first, second}))
			},
			want: []entity.ValidationFact{first, second},
		},
		{
			name:  "retained facts include original owner and degree",
			input: []entity.ValidationFact{first, second},
			setup: func(mock sqlmock.Sqlmock) {
				expectFactBatchInsert(mock, []entity.ValidationFact{first, second}).WillReturnResult(sqlmock.NewResult(0, 1))
				expectFactBatchRead(mock, []entity.ValidationFact{first, second}).
					WillReturnRows(factBatchRows([]entity.ValidationFact{retained, second}))
			},
			want: []entity.ValidationFact{retained, second},
		},
		{
			name:  "input order does not depend on read order",
			input: []entity.ValidationFact{first, second},
			setup: func(mock sqlmock.Sqlmock) {
				expectFactBatchInsert(mock, []entity.ValidationFact{first, second}).WillReturnResult(sqlmock.NewResult(0, 2))
				rows := sqlmock.NewRows([]string{"position", "uri", "project", "project_type", "degree", "request_id", "created_at"}).
					AddRow(1, second.URI, second.Project, second.ProjectType, second.Degree, second.RequestID, second.CreatedAt).
					AddRow(0, first.URI, first.Project, first.ProjectType, first.Degree, first.RequestID, first.CreatedAt)
				expectFactBatchRead(mock, []entity.ValidationFact{first, second}).WillReturnRows(rows)
			},
			want: []entity.ValidationFact{first, second},
		},
		{
			name:  "SQL errors other than duplicates are not ignored",
			input: []entity.ValidationFact{first},
			setup: func(mock sqlmock.Sqlmock) {
				expectFactBatchInsert(mock, []entity.ValidationFact{first}).WillReturnError(writeErr)
			},
			wantErr: writeErr,
		},
		{
			name:  "read failure after durable insert",
			input: []entity.ValidationFact{first},
			setup: func(mock sqlmock.Sqlmock) {
				expectFactBatchInsert(mock, []entity.ValidationFact{first}).WillReturnResult(sqlmock.NewResult(0, 1))
				expectFactBatchRead(mock, []entity.ValidationFact{first}).WillReturnError(readErr)
			},
			wantErr: readErr,
		},
		{
			name:  "row read failure",
			input: []entity.ValidationFact{first},
			setup: func(mock sqlmock.Sqlmock) {
				expectFactBatchInsert(mock, []entity.ValidationFact{first}).WillReturnResult(sqlmock.NewResult(0, 1))
				expectFactBatchRead(mock, []entity.ValidationFact{first}).
					WillReturnRows(factBatchRows([]entity.ValidationFact{first}).RowError(0, readErr))
			},
			wantErr: readErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, store := setupValidationFactStoreTest(t)
			t.Cleanup(func() { _ = db.Close() })
			if tt.setup != nil {
				tt.setup(mock)
			}
			original := append([]entity.ValidationFact(nil), tt.input...)
			got, err := store.CreateBatch(context.Background(), tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			assert.Equal(t, original, tt.input)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestValidationFactStore_CreateBatchReadValidation(t *testing.T) {
	fact := entity.ValidationFact{URI: testFactURI, Project: "one", RequestID: "1", CreatedAt: 10}
	tests := []struct {
		name string
		rows *sqlmock.Rows
	}{
		{
			name: "missing retained fact",
			rows: factBatchRows(nil),
		},
		{
			name: "invalid stored value",
			rows: sqlmock.NewRows([]string{"position", "uri", "project", "project_type", "degree", "request_id", "created_at"}).
				AddRow(0, fact.URI, fact.Project, "", "invalid-degree", fact.RequestID, fact.CreatedAt),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, store := setupValidationFactStoreTest(t)
			t.Cleanup(func() { _ = db.Close() })
			expectFactBatchInsert(mock, []entity.ValidationFact{fact}).WillReturnResult(sqlmock.NewResult(0, 1))
			expectFactBatchRead(mock, []entity.ValidationFact{fact}).WillReturnRows(tt.rows)
			got, err := store.CreateBatch(context.Background(), []entity.ValidationFact{fact})
			require.Error(t, err)
			assert.Nil(t, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestValidationFactStore_CreateBatchRetryAfterPartialWrite(t *testing.T) {
	db, mock, store := setupValidationFactStoreTest(t)
	t.Cleanup(func() { _ = db.Close() })
	facts := make([]entity.ValidationFact, validationFactBatchSize+1)
	for i := range facts {
		facts[i] = entity.ValidationFact{URI: testFactURI, Project: fmt.Sprintf("project-%03d", i), RequestID: "1", CreatedAt: 10}
	}
	first, last := facts[:validationFactBatchSize], facts[validationFactBatchSize:]
	writeErr := errors.New("connection lost")
	expectFactBatchInsert(mock, first).WillReturnResult(sqlmock.NewResult(0, int64(len(first))))
	expectFactBatchRead(mock, first).WillReturnRows(factBatchRows(first))
	expectFactBatchInsert(mock, last).WillReturnError(writeErr)
	got, err := store.CreateBatch(context.Background(), facts)
	require.ErrorIs(t, err, writeErr)
	assert.Nil(t, got)

	expectFactBatchInsert(mock, first).WillReturnResult(sqlmock.NewResult(0, 0))
	expectFactBatchRead(mock, first).WillReturnRows(factBatchRows(first))
	expectFactBatchInsert(mock, last).WillReturnResult(sqlmock.NewResult(0, 1))
	expectFactBatchRead(mock, last).WillReturnRows(factBatchRows(last))
	got, err = store.CreateBatch(context.Background(), facts)
	require.NoError(t, err)
	assert.Equal(t, facts, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectFactBatchInsert(mock sqlmock.Sqlmock, facts []entity.ValidationFact) *sqlmock.ExpectedExec {
	args := make([]driver.Value, 0, len(facts)*7)
	for _, fact := range facts {
		args = append(args, testFactQueue, fact.URI, fact.Project, fact.ProjectType, fact.Degree, fact.RequestID, fact.CreatedAt)
	}
	return mock.ExpectExec("INSERT INTO validation_fact").WithArgs(args...)
}

func expectFactBatchRead(mock sqlmock.Sqlmock, facts []entity.ValidationFact) *sqlmock.ExpectedQuery {
	args := make([]driver.Value, 0, len(facts)*4)
	for i, fact := range facts {
		args = append(args, i, testFactQueue, fact.URI, fact.Project)
	}
	return mock.ExpectQuery("SELECT").WithArgs(args...)
}

func factBatchRows(facts []entity.ValidationFact) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"position", "uri", "project", "project_type", "degree", "request_id", "created_at"})
	for i, fact := range facts {
		rows.AddRow(i, fact.URI, fact.Project, fact.ProjectType, fact.Degree, fact.RequestID, fact.CreatedAt)
	}
	return rows
}
