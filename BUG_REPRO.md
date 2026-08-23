# Bug Reproduction

## Bug

A calibration or fault work order can complete without required evidence and still trigger crane state changes.

## Trigger

Create an eligible work order with no evidence references and request completion with the stop-crane workflow.

## Observed error

The work order becomes `completed` and a stop side effect is recorded instead of returning an evidence-required error with no state change.
