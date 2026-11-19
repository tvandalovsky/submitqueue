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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/stovepipe/entity"
	mysqlstorage "github.com/uber/submitqueue/stovepipe/extension/storage/mysql"
)

func testValidationFactBatchRetry(t *testing.T, ctx context.Context, backend *mysqlstorage.Storage) {
	bound, err := backend.For("fact-batch-retry")
	require.NoError(t, err)
	store := bound.GetValidationFactStore()
	facts := make([]entity.ValidationFact, 201)
	for i := range facts {
		facts[i] = entity.ValidationFact{URI: "commit", Project: fmt.Sprintf("project-%03d", i), RequestID: "1", CreatedAt: 10}
	}
	facts[200].Project = strings.Repeat("x", 256)
	_, err = store.CreateBatch(ctx, facts)
	require.Error(t, err)
	retained, err := store.Get(ctx, facts[0].URI, facts[0].Project)
	require.NoError(t, err)
	assert.Equal(t, facts[0], retained)

	facts[200].Project = "repaired"
	for i := range facts {
		facts[i].CreatedAt = 20
	}
	got, err := store.CreateBatch(ctx, facts)
	require.NoError(t, err)
	for i, fact := range got {
		expected := facts[i]
		if i < 200 {
			expected.CreatedAt = 10
		}
		assert.Equal(t, expected, fact)
	}
}

func testValidationFactBatchCollation(t *testing.T, ctx context.Context, backend *mysqlstorage.Storage) {
	bound, err := backend.For("fact-batch-collation")
	require.NoError(t, err)
	store := bound.GetValidationFactStore()
	first := entity.ValidationFact{URI: "commit", Project: "Project", Degree: 1, RequestID: "1", CreatedAt: 10}
	require.NoError(t, store.Create(ctx, first))
	alias := first
	alias.Project = "project"
	alias.Degree = 0
	alias.RequestID = "2"
	got, err := store.CreateBatch(ctx, []entity.ValidationFact{alias, first})
	require.NoError(t, err)
	assert.Equal(t, []entity.ValidationFact{first, first}, got)
}
