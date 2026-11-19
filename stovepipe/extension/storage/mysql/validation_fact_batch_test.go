// Copyright (c) 2026 Uber Technologies, Inc.
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
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/stovepipe/entity"
)

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
