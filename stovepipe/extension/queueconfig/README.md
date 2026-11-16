# Queue configuration

Queue configuration is deployment policy, separate from mutable queue state in storage. The provider interface supplies immutable, fully resolved settings by queue name; it does not imply database persistence. Controllers obtain one configuration snapshot per delivery without knowing how it was loaded.

## Stage ownership

Configuration is grouped by stage: process, build signal, build analysis, and issue tracking. Each stage owns its resolved entity and validation. Source-independent optional overrides live in `stovepipe/core/queueconfig`, with stage-specific application helpers that copy values, convert standard durations to whole milliseconds, and replace only explicitly supplied fields. Providers can use these helpers but are not required to expose or use overrides. Aggregates compose stages rather than inspecting individual settings.

The process stage owns concurrency and admission waits. The other current stage settings control build-status polling, build-issue analysis polling, and issue-tracking refresh and recovery waits. Stage configuration can contain both timing and non-timing settings. Immediate batch continuation and consumer error-retry policies remain separate.

## YAML provider

The YAML implementation decodes the shared optional configuration types directly and retains an immutable snapshot of one file read at construction. Shared stage defaults and explicit named queues are declared in that file. Queue-specific settings override shared defaults; omitted values inherit while explicit values follow the owning stage's constraints. Only the resolved queue configurations undergo semantic validation, allowing queues to complete partial defaults. The current concurrency limits and millisecond intervals must be positive.

The provider rejects malformed configuration, unknown fields, duplicate keys or queue names, missing required settings, and multiple YAML documents. It serves only declared queues, returns independent values, and preserves declaration order when listing. It does not watch files, merge multiple files, or supply hardcoded operational defaults.

## Host integration

Hosts choose and construct the provider and inject the lookup interface. The example server selects a single `queues.yaml`; e2e selects a separate file through the same path. A host using another configuration framework can return the same resolved values without adopting the YAML schema or its defaults mechanism. Messaging tenant registration remains a separate host concern.

Adding a setting requires updating its stage's resolved value, validation, and optional input with its application logic. The YAML provider does not repeat stage schemas or field mappings. It does not require field-level logic in the lookup interface or the aggregate.
