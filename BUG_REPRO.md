# Bug Reproduction

## Bug

A refresh token with a valid identifier but a forged secret is accepted and rotated.

## Trigger

Issue a refresh token, keep its identifier prefix, replace the secret suffix, and submit the forged credential for refresh.

## Observed error

New access and refresh tokens are issued, and the original legitimate refresh token is revoked instead of the forged credential being rejected.
