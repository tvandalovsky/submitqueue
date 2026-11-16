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

// Package yaml provides a startup snapshot of one queue configuration file.
// Shared defaults and per-queue overrides are resolved before validation.
package yaml

import (
	"context"
	"fmt"
	"io"
	"os"

	configlayer "github.com/uber/submitqueue/stovepipe/core/queueconfig"
	"github.com/uber/submitqueue/stovepipe/entity"
	"github.com/uber/submitqueue/stovepipe/extension/queueconfig"
	yamlv3 "gopkg.in/yaml.v3"
)

type document struct {
	Defaults stageSection    `yaml:"defaults"`
	Queues   []queueSettings `yaml:"queues"`
}

type stageSection struct {
	Stages configlayer.Overrides `yaml:"stages"`
}

type queueSettings struct {
	Name         string `yaml:"name"`
	stageSection `yaml:",inline"`
}

// Store holds immutable, resolved settings for explicitly declared queues.
// Get and List return independent values and are safe for concurrent use.
type Store struct {
	byName map[string]entity.QueueConfig
	all    []entity.QueueConfig
}

var _ queueconfig.Store = Store{}

// NewStore reads exactly one file at startup; the file is not watched or layered
// over an implicit baseline. Invalid configuration prevents construction.
func NewStore(path string) (Store, error) {
	file, err := os.Open(path)
	if err != nil {
		return Store{}, fmt.Errorf("open queue configuration %q: %w", path, err)
	}
	defer file.Close()

	cfg, err := decodeDocument(file)
	if err != nil {
		return Store{}, fmt.Errorf("decode queue configuration %q: %w", path, err)
	}

	store, err := resolveQueueConfigs(cfg)
	if err != nil {
		return Store{}, fmt.Errorf("resolve queue configuration %q: %w", path, err)
	}

	return store, nil
}

func decodeDocument(reader io.Reader) (document, error) {
	decoder := yamlv3.NewDecoder(reader)
	decoder.KnownFields(true)

	var cfg document
	if err := decoder.Decode(&cfg); err != nil {
		return document{}, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return document{}, err
		}

		return document{}, fmt.Errorf("configuration must contain exactly one YAML document")
	}

	return cfg, nil
}

func resolveQueueConfigs(cfg document) (Store, error) {
	base, err := cfg.Defaults.Stages.Apply(entity.StageConfig{})
	if err != nil {
		return Store{}, fmt.Errorf("defaults: %w", err)
	}

	if len(cfg.Queues) == 0 {
		return Store{}, fmt.Errorf("configuration must declare at least one queue")
	}

	store := Store{
		byName: make(map[string]entity.QueueConfig, len(cfg.Queues)),
		all:    make([]entity.QueueConfig, 0, len(cfg.Queues)),
	}

	for _, settings := range cfg.Queues {
		if _, exists := store.byName[settings.Name]; exists {
			return Store{}, fmt.Errorf("duplicate queue %q", settings.Name)
		}

		stages, err := settings.Stages.Apply(base)
		if err != nil {
			return Store{}, fmt.Errorf("queue %q: %w", settings.Name, err)
		}

		resolved := entity.QueueConfig{
			Name:   settings.Name,
			Stages: stages,
		}

		if err := resolved.Validate(); err != nil {
			return Store{}, err
		}

		store.byName[resolved.Name] = resolved
		store.all = append(store.all, resolved)
	}

	return store, nil
}

// Get returns ErrNotFound for queues not declared in the file.
func (s Store) Get(_ context.Context, name string) (entity.QueueConfig, error) {
	cfg, ok := s.byName[name]
	if !ok {
		return entity.QueueConfig{}, queueconfig.ErrNotFound
	}

	return cfg, nil
}

// List returns independent values in declaration order.
func (s Store) List(context.Context) ([]entity.QueueConfig, error) {
	configs := make([]entity.QueueConfig, len(s.all))
	copy(configs, s.all)

	return configs, nil
}
