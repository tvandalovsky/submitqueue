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
	"fmt"
	"time"

	"github.com/uber/submitqueue/stovepipe/entity"
)

// IssueTrackOverrides contains optional configuration for the issue-tracking stage.
// Nil fields inherit their previous value; explicit values are retained for validation.
type IssueTrackOverrides struct {
	// RefreshDelay overrides the interval between refreshes using whole milliseconds.
	RefreshDelay *time.Duration `yaml:"refreshDelay"`

	// RecoveryWaitDelay overrides the wait during recovery using whole milliseconds.
	RecoveryWaitDelay *time.Duration `yaml:"recoveryWaitDelay"`
}

// Apply copies base and replaces explicitly supplied settings.
// Durations that cannot be represented as whole milliseconds return an error.
func (o IssueTrackOverrides) Apply(base entity.IssueTrackConfig) (entity.IssueTrackConfig, error) {
	refreshDelayMs, err := durationMilliseconds(o.RefreshDelay, base.RefreshDelayMs)
	if err != nil {
		return entity.IssueTrackConfig{}, fmt.Errorf("refreshDelay: %w", err)
	}

	base.RefreshDelayMs = refreshDelayMs

	recoveryWaitDelayMs, err := durationMilliseconds(o.RecoveryWaitDelay, base.RecoveryWaitDelayMs)
	if err != nil {
		return entity.IssueTrackConfig{}, fmt.Errorf("recoveryWaitDelay: %w", err)
	}

	base.RecoveryWaitDelayMs = recoveryWaitDelayMs

	return base, nil
}
