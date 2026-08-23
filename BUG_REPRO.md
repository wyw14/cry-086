# Bug Reproduction

## Bug

A load reading older than the configured reorder window is accepted as current telemetry.

## Trigger

Ingest a valid reading for one sensor, then submit a lower sequence whose distance from the latest sequence exceeds the reorder allowance.

## Observed error

The ingestion response reports success instead of isolating the stale reading, so the dashboard can consume an obsolete value.
