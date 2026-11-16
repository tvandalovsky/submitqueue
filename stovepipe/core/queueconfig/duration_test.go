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
)

func TestDurationMilliseconds(t *testing.T) {
	inherited, err := durationMilliseconds(nil, 123)
	require.NoError(t, err)
	assert.Equal(t, int64(123), inherited)

	for _, tt := range []struct {
		name    string
		value   time.Duration
		want    int64
		wantErr bool
	}{
		{"milliseconds", 250 * time.Millisecond, 250, false},
		{"seconds", 2 * time.Second, 2000, false},
		{"zero is explicit", 0, 0, false},
		{"negative retained for validation", -time.Second, -1000, false},
		{"submillisecond", time.Microsecond, 0, true},
		{"fractional millisecond", time.Millisecond + time.Microsecond, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := durationMilliseconds(&tt.value, 123)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
