$ErrorActionPreference = 'Stop'
if (-not $env:ACCESS_TOKEN_SECRET) { $env:ACCESS_TOKEN_SECRET = 'local-development-secret-change-me' }
if (-not $env:SIMULATOR_KEY) { $env:SIMULATOR_KEY = 'local-simulator-key' }
if (-not $env:REPOSITORY_MODE) { $env:REPOSITORY_MODE = 'memory' }
go run ./cmd/server
