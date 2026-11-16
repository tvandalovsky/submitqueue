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

	"github.com/uber/submitqueue/stovepipe/entity"
)

// Overrides contains optional configuration grouped by stage.
// Apply does not validate partial layers; validate the resolved result instead.
type Overrides struct {
	// Process contains optional configuration for the process stage.
	Process ProcessOverrides `yaml:"process"`

	// BuildSignal contains optional configuration for the build-signal stage.
	BuildSignal BuildSignalOverrides `yaml:"buildSignal"`

	// BuildAnalysis contains optional configuration for the build-analysis stage.
	BuildAnalysis BuildAnalysisOverrides `yaml:"buildAnalysis"`

	// IssueTrack contains optional configuration for the issue-tracking stage.
	IssueTrack IssueTrackOverrides `yaml:"issueTrack"`
}

// Apply copies base and applies each stage's overrides.
// Durations that cannot be represented as whole milliseconds return an error.
func (o Overrides) Apply(base entity.StageConfig) (entity.StageConfig, error) {
	process, err := o.Process.Apply(base.Process)
	if err != nil {
		return entity.StageConfig{}, fmt.Errorf("process: %w", err)
	}

	buildSignal, err := o.BuildSignal.Apply(base.BuildSignal)
	if err != nil {
		return entity.StageConfig{}, fmt.Errorf("buildSignal: %w", err)
	}

	buildAnalysis, err := o.BuildAnalysis.Apply(base.BuildAnalysis)
	if err != nil {
		return entity.StageConfig{}, fmt.Errorf("buildAnalysis: %w", err)
	}

	issueTrack, err := o.IssueTrack.Apply(base.IssueTrack)
	if err != nil {
		return entity.StageConfig{}, fmt.Errorf("issueTrack: %w", err)
	}

	return entity.StageConfig{
		Process:       process,
		BuildSignal:   buildSignal,
		BuildAnalysis: buildAnalysis,
		IssueTrack:    issueTrack,
	}, nil
}
