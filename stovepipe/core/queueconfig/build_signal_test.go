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

func TestBuildSignalOverridesApply(t *testing.T) {
	base := entity.BuildSignalConfig{
		AcceptedPollDelayMs: 10,
		RunningPollDelayMs:  11,
	}

	inherited, err := (BuildSignalOverrides{}).Apply(base)
	require.NoError(t, err)
	assert.Equal(t, base, inherited)

	t.Run("AcceptedPollDelay", func(t *testing.T) {
		value := 25 * time.Millisecond
		original := base
		override := BuildSignalOverrides{AcceptedPollDelay: &value}

		got, err := override.Apply(base)
		require.NoError(t, err)
		want := base
		want.AcceptedPollDelayMs = value.Milliseconds()

		assert.Equal(t, want, got)
		assert.Equal(t, original, base)

		value = 0
		assert.Equal(t, int64(25), got.AcceptedPollDelayMs)

		zero, err := override.Apply(base)
		require.NoError(t, err)
		assert.Zero(t, zero.AcceptedPollDelayMs)
	})

	t.Run("RunningPollDelay", func(t *testing.T) {
		value := 25 * time.Millisecond
		original := base
		override := BuildSignalOverrides{RunningPollDelay: &value}

		got, err := override.Apply(base)
		require.NoError(t, err)
		want := base
		want.RunningPollDelayMs = value.Milliseconds()

		assert.Equal(t, want, got)
		assert.Equal(t, original, base)

		value = 0
		assert.Equal(t, int64(25), got.RunningPollDelayMs)

		zero, err := override.Apply(base)
		require.NoError(t, err)
		assert.Zero(t, zero.RunningPollDelayMs)
	})
}
