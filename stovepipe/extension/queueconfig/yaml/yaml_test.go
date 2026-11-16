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

package yaml

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/stovepipe/extension/queueconfig"
)

const testDefaults = `defaults:
  stages:
    process:
      maxConcurrent: 3
      gateWaitDelay: 123ms
    buildSignal:
      acceptedPollDelay: 234ms
      runningPollDelay: 345ms
    buildAnalysis:
      pollDelay: 456ms
    issueTrack:
      refreshDelay: 567ms
      recoveryWaitDelay: 678ms
`

func writeTestConfig(t *testing.T, input string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "queues.yaml")
	require.NoError(t, os.WriteFile(path, []byte(input), 0600))
	return path
}

func TestStageAndQueueOverrides(t *testing.T) {
	store, err := NewStore(writeTestConfig(t, testDefaults+`queues:
  - name: special
    stages:
      process:
        maxConcurrent: 4
      buildAnalysis:
        pollDelay: 2s
  - name: ordinary
`))
	require.NoError(t, err)

	ordinary, err := store.Get(context.Background(), "ordinary")
	require.NoError(t, err)

	special, err := store.Get(context.Background(), "special")
	require.NoError(t, err)

	assert.Equal(t, int32(3), ordinary.Stages.Process.MaxConcurrent)
	assert.Equal(t, int64(123), ordinary.Stages.Process.GateWaitDelayMs)
	assert.Equal(t, int64(234), ordinary.Stages.BuildSignal.AcceptedPollDelayMs)
	assert.Equal(t, int64(345), ordinary.Stages.BuildSignal.RunningPollDelayMs)
	assert.Equal(t, int64(456), ordinary.Stages.BuildAnalysis.PollDelayMs)
	assert.Equal(t, int64(567), ordinary.Stages.IssueTrack.RefreshDelayMs)
	assert.Equal(t, int64(678), ordinary.Stages.IssueTrack.RecoveryWaitDelayMs)

	assert.Equal(t, int32(4), special.Stages.Process.MaxConcurrent)
	assert.Equal(t, ordinary.Stages.Process.GateWaitDelayMs, special.Stages.Process.GateWaitDelayMs)
	assert.Equal(t, int64(2000), special.Stages.BuildAnalysis.PollDelayMs)
	assert.Equal(t, ordinary.Stages.BuildSignal, special.Stages.BuildSignal)
	assert.Equal(t, ordinary.Stages.IssueTrack, special.Stages.IssueTrack)
}

func TestValidationUsesResolvedQueueSettings(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
	}{
		{
			name: "queue completes partial defaults",
			input: strings.Replace(testDefaults, "      pollDelay: 456ms\n", "", 1) + `queues:
  - name: known
    stages:
      buildAnalysis:
        pollDelay: 2s
`,
		},
		{
			name: "queue overrides invalid default",
			input: strings.Replace(testDefaults, "pollDelay: 456ms", "pollDelay: 0s", 1) + `queues:
  - name: known
    stages:
      buildAnalysis:
        pollDelay: 2s
`,
		},
		{
			name:  "complete queue does not need defaults",
			input: "queues:\n  - name: known\n  " + strings.ReplaceAll(strings.TrimPrefix(testDefaults, "defaults:\n"), "\n", "\n  "),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(writeTestConfig(t, tt.input))
			require.NoError(t, err)

			cfg, err := store.Get(context.Background(), "known")
			require.NoError(t, err)
			require.NoError(t, cfg.Validate())
		})
	}
}

func TestRejectsInvalidConfiguration(t *testing.T) {
	valid := testDefaults + "queues:\n  - name: known\n"

	for _, tt := range []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"empty document", "{}"},
		{"missing queues", testDefaults},
		{"empty queues", testDefaults + "queues: []"},
		{"missing settings", "queues: [{name: known}]"},
		{"unknown section", valid + "default: {}"},
		{"unknown defaults field", strings.Replace(valid, "  stages:", "  stagez:", 1)},
		{"unknown stage", strings.Replace(valid, "    process:", "    unknown:", 1)},
		{"unknown process field", strings.Replace(valid, "maxConcurrent:", "maxConcurent:", 1)},
		{"unknown signal field", strings.Replace(valid, "acceptedPollDelay:", "pollDelay:", 1)},
		{"unknown analysis field", strings.Replace(valid, "pollDelay:", "polDelay:", 1)},
		{"unknown tracking field", strings.Replace(valid, "refreshDelay:", "pollDelay:", 1)},
		{"unknown queue field", valid + "    stagez: {}"},
		{"unknown queue stage", valid + "    stages: {unknown: {}}"},
		{"duplicate key", strings.Replace(valid, "maxConcurrent: 3", "maxConcurrent: 3\n      maxConcurrent: 4", 1)},
		{"duplicate queue", valid + "  - name: known\n"},
		{"empty name", strings.Replace(valid, "name: known", "name: ''", 1)},
		{"missing name", testDefaults + "queues: [{}]"},
		{"zero concurrency", strings.Replace(valid, "maxConcurrent: 3", "maxConcurrent: 0", 1)},
		{"zero wait", strings.Replace(valid, "pollDelay: 456ms", "pollDelay: 0s", 1)},
		{"negative wait", strings.Replace(valid, "refreshDelay: 567ms", "refreshDelay: -1s", 1)},
		{"invalid duration", strings.Replace(valid, "gateWaitDelay: 123ms", "gateWaitDelay: soon", 1)},
		{"submillisecond", strings.Replace(valid, "acceptedPollDelay: 234ms", "acceptedPollDelay: 1us", 1)},
		{"fractional millisecond", strings.Replace(valid, "recoveryWaitDelay: 678ms", "recoveryWaitDelay: 1500us", 1)},
		{"invalid queue override", valid + "    stages: {process: {gateWaitDelay: 0s}}"},
		{"incomplete sibling queue", strings.Replace(testDefaults, "      pollDelay: 456ms\n", "", 1) + `queues:
  - name: complete
    stages: {buildAnalysis: {pollDelay: 1s}}
  - name: incomplete
`},
		{"second document", valid + "---\n{}"},
		{"malformed second document", valid + "---\n["},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStore(writeTestConfig(t, tt.input))
			require.Error(t, err)
		})
	}
}

func TestStoreSnapshotAndIsolation(t *testing.T) {
	ctx := context.Background()
	path := writeTestConfig(t, testDefaults+"queues: [{name: second}, {name: first}]")
	store, err := NewStore(path)
	require.NoError(t, err)

	want, err := store.Get(ctx, "second")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("invalid replacement"), 0600))

	got := want
	got.Stages.Process.MaxConcurrent = 99

	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, []string{"second", "first"}, []string{listed[0].Name, listed[1].Name})
	listed[0] = got

	again, err := store.Get(ctx, "second")
	require.NoError(t, err)
	assert.Equal(t, want, again)

	listedAgain, err := store.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, listedAgain[0])

	for _, name := range []string{"", "unknown"} {
		_, err := store.Get(ctx, name)
		require.ErrorIs(t, err, queueconfig.ErrNotFound)
	}
}

func TestMissingFileFails(t *testing.T) {
	_, err := NewStore(filepath.Join(t.TempDir(), "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
