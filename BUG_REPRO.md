# Bug Reproduction

## Bug

A crane is evaluated with a staged safety configuration version that is not yet bound to the crane.

## Trigger

Keep a crane bound to load-curve version 3, stage version 4 under the same configuration ID, and evaluate a load that is critical under version 3 only.

## Observed error

The decision records configuration version 4 and returns a normal risk level instead of the required version-3 overload interlock.
