# Bug Reproduction

## Bug

Reading the same event replay window mutates retained evidence.

## Trigger

Replay one site's identical time interval twice after storing an auditable timeline event.

## Observed error

The second response contains different replay counters or ordering metadata, so serialized evidence differs between two read-only requests.
