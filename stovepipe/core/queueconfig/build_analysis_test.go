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

package queueconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/stovepipe/entity"
)

func TestBuildAnalysisOverridesApply(t *testing.T) {
	base := entity.BuildAnalysisConfig{
		PollDelayMs: 10,
	}

	inherited, err := (BuildAnalysisOverrides{}).Apply(base)
	require.NoError(t, err)
	assert.Equal(t, base, inherited)

	t.Run("PollDelay", func(t *testing.T) {
		value := 25 * time.Millisecond
		original := base
		override := BuildAnalysisOverrides{PollDelay: &value}

		got, err := override.Apply(base)
		require.NoError(t, err)
		want := base
		want.PollDelayMs = value.Milliseconds()

		assert.Equal(t, want, got)
		assert.Equal(t, original, base)

		value = 0
		assert.Equal(t, int64(25), got.PollDelayMs)

		zero, err := override.Apply(base)
		require.NoError(t, err)
		assert.Zero(t, zero.PollDelayMs)
	})
}
