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

func TestOverridesComposeAllStages(t *testing.T) {
	concurrency := int32(2)
	admission, accepted, running := 11*time.Millisecond, 12*time.Millisecond, 13*time.Millisecond
	analysis, refresh, recovery := 14*time.Millisecond, 15*time.Millisecond, 16*time.Millisecond
	overrides := Overrides{
		Process:       ProcessOverrides{MaxConcurrent: &concurrency, GateWaitDelay: &admission},
		BuildSignal:   BuildSignalOverrides{AcceptedPollDelay: &accepted, RunningPollDelay: &running},
		BuildAnalysis: BuildAnalysisOverrides{PollDelay: &analysis},
		IssueTrack:    IssueTrackOverrides{RefreshDelay: &refresh, RecoveryWaitDelay: &recovery},
	}
	var base entity.StageConfig

	got, err := overrides.Apply(base)
	require.NoError(t, err)
	require.NoError(t, got.Validate())
	assert.Equal(t, entity.StageConfig{}, base)
	assert.Equal(t, concurrency, got.Process.MaxConcurrent)
	assert.Equal(t, admission.Milliseconds(), got.Process.GateWaitDelayMs)
	assert.Equal(t, accepted.Milliseconds(), got.BuildSignal.AcceptedPollDelayMs)
	assert.Equal(t, running.Milliseconds(), got.BuildSignal.RunningPollDelayMs)
	assert.Equal(t, analysis.Milliseconds(), got.BuildAnalysis.PollDelayMs)
	assert.Equal(t, refresh.Milliseconds(), got.IssueTrack.RefreshDelayMs)
	assert.Equal(t, recovery.Milliseconds(), got.IssueTrack.RecoveryWaitDelayMs)

	inherited, err := (Overrides{}).Apply(got)
	require.NoError(t, err)
	assert.Equal(t, got, inherited)
}

func TestOverridesRejectFractionalMilliseconds(t *testing.T) {
	fractional := time.Microsecond

	for _, tt := range []struct {
		name      string
		overrides Overrides
	}{
		{"process", Overrides{Process: ProcessOverrides{GateWaitDelay: &fractional}}},
		{"build signal", Overrides{BuildSignal: BuildSignalOverrides{RunningPollDelay: &fractional}}},
		{"build analysis", Overrides{BuildAnalysis: BuildAnalysisOverrides{PollDelay: &fractional}}},
		{"issue tracking", Overrides{IssueTrack: IssueTrackOverrides{RecoveryWaitDelay: &fractional}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.overrides.Apply(entity.StageConfig{})
			require.Error(t, err)
		})
	}
}
