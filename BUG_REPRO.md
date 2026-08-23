# Bug Reproduction

## Bug

An interlock can be resolved before the crane's configured recovery hold has elapsed.

## Trigger

Begin recovery for a critical alarm and submit a client hold value shorter than the safety configuration's authoritative hold.

## Observed error

The alarm moves to `resolved` even though the configured continuous-safe interval is still incomplete.
