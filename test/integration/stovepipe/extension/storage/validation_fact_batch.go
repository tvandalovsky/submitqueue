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

package storage

import (
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/stovepipe/entity"
)

func (s *ValidationFactStoreContractSuite) TestValidationFactStore_CreateBatch() {
	first := entity.ValidationFact{URI: "commit-a", Project: "service-a", ProjectType: "service", Degree: 0.25, RequestID: "1", CreatedAt: 10}
	second := entity.ValidationFact{URI: "commit-a", Project: "library-b", ProjectType: "library", Degree: 1, RequestID: "1", CreatedAt: 20}
	repository := entity.ValidationFact{URI: "commit-b", Degree: 0, RequestID: "2", CreatedAt: 30}
	changed := first
	changed.ProjectType = "different-type"
	changed.Degree = 1
	changed.RequestID = "other-request"
	changed.CreatedAt = 999
	tests := []struct {
		name     string
		existing []entity.ValidationFact
		input    []entity.ValidationFact
		want     []entity.ValidationFact
	}{
		{name: "empty"},
		{
			name:  "multiple commits and repository facts",
			input: []entity.ValidationFact{first, second, repository},
			want:  []entity.ValidationFact{first, second, repository},
		},
		{
			name:     "mixed existing and new",
			existing: []entity.ValidationFact{first},
			input:    []entity.ValidationFact{changed, second},
			want:     []entity.ValidationFact{first, second},
		},
		{
			name:     "all existing",
			existing: []entity.ValidationFact{first, second},
			input:    []entity.ValidationFact{second, changed},
			want:     []entity.ValidationFact{second, first},
		},
		{
			name:  "repeated identities keep first input",
			input: []entity.ValidationFact{first, second, changed},
			want:  []entity.ValidationFact{first, second, first},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()
			store := s.storeFor("contract/fact-batch/" + tt.name)
			for _, fact := range tt.existing {
				require.NoError(t, store.Create(s.ctx, fact))
			}
			inputCopy := append([]entity.ValidationFact(nil), tt.input...)
			for range 2 {
				got, err := store.CreateBatch(s.ctx, tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				assert.Equal(t, inputCopy, tt.input)
				for _, expected := range tt.want {
					stored, err := store.Get(s.ctx, expected.URI, expected.Project)
					require.NoError(t, err)
					assert.Equal(t, expected, stored)
				}
			}
		})
	}
}

func (s *ValidationFactStoreContractSuite) TestValidationFactStore_CreateBatchLargeAndIsolated() {
	t := s.T()
	facts := make([]entity.ValidationFact, 501)
	for i := range facts {
		facts[i] = entity.ValidationFact{
			URI: "commit", Project: fmt.Sprintf("project-%03d", i), ProjectType: "service",
			Degree: float64(i % 2), RequestID: "1", CreatedAt: int64(i + 1),
		}
	}
	otherStore := s.storeFor("contract/fact-batch/other-queue")
	other := facts[0]
	other.RequestID = "other-request"
	other.Degree = 1
	require.NoError(t, otherStore.Create(s.ctx, other))

	store := s.storeFor("contract/fact-batch/large")
	for range 2 {
		got, err := store.CreateBatch(s.ctx, facts)
		require.NoError(t, err)
		assert.Equal(t, facts, got)
	}
	got, err := store.ListProjects(s.ctx, facts[0].URI, "", len(facts)+1)
	require.NoError(t, err)
	assert.Equal(t, facts, got)
	retained, err := otherStore.Get(s.ctx, other.URI, other.Project)
	require.NoError(t, err)
	assert.Equal(t, other, retained)
}

func (s *ValidationFactStoreContractSuite) TestValidationFactStore_CreateBatchConcurrentWriters() {
	t := s.T()
	store := s.storeFor("contract/fact-batch/concurrent")
	first := []entity.ValidationFact{
		{URI: "commit", Project: "a", Degree: 0, RequestID: "1", CreatedAt: 10},
		{URI: "commit", Project: "b", Degree: 1, RequestID: "1", CreatedAt: 10},
	}
	second := append([]entity.ValidationFact(nil), first...)
	for i := range second {
		second[i].RequestID = "2"
		second[i].Degree = 0.5
	}
	type result struct {
		facts []entity.ValidationFact
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, 1)
	for _, input := range [][]entity.ValidationFact{first, second} {
		go func() {
			<-start
			facts, err := store.CreateBatch(s.ctx, input)
			results <- result{facts: facts, err: err}
		}()
	}
	close(start)
	for range 2 {
		got := <-results
		require.NoError(t, got.err)
		require.Len(t, got.facts, len(first))
		for i, fact := range got.facts {
			retained, err := store.Get(s.ctx, first[i].URI, first[i].Project)
			require.NoError(t, err)
			assert.Equal(t, retained, fact)
			assert.Contains(t, []entity.ValidationFact{first[i], second[i]}, fact)
		}
	}
}
