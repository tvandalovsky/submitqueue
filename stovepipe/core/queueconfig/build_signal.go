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

// BuildSignalOverrides contains optional configuration for the build-signal stage.
// Nil fields inherit their previous value; explicit values are retained for validation.
type BuildSignalOverrides struct {
	// AcceptedPollDelay overrides the polling interval for accepted builds using whole milliseconds.
	AcceptedPollDelay *time.Duration `yaml:"acceptedPollDelay"`

	// RunningPollDelay overrides the polling interval for running builds using whole milliseconds.
	RunningPollDelay *time.Duration `yaml:"runningPollDelay"`
}

// Apply copies base and replaces explicitly supplied settings.
// Durations that cannot be represented as whole milliseconds return an error.
func (o BuildSignalOverrides) Apply(base entity.BuildSignalConfig) (entity.BuildSignalConfig, error) {
	acceptedPollDelayMs, err := durationMilliseconds(o.AcceptedPollDelay, base.AcceptedPollDelayMs)
	if err != nil {
		return entity.BuildSignalConfig{}, fmt.Errorf("acceptedPollDelay: %w", err)
	}

	base.AcceptedPollDelayMs = acceptedPollDelayMs

	runningPollDelayMs, err := durationMilliseconds(o.RunningPollDelay, base.RunningPollDelayMs)
	if err != nil {
		return entity.BuildSignalConfig{}, fmt.Errorf("runningPollDelay: %w", err)
	}

	base.RunningPollDelayMs = runningPollDelayMs

	return base, nil
}
