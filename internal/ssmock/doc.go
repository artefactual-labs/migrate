// Package ssmock provides an in-memory mock implementation of the Archivematica
// Storage Service API for testing purposes.
//
// ## Purpose
//
// The ssmock package simulates the Storage Service HTTP API and management
// commands, allowing end-to-end tests to run without requiring a full
// Archivematica deployment.
//
// ## Architecture
//
// The mock server maintains an in-memory state that models Storage Service
// locations and packages (AIPs). It implements the relevant REST endpoints
// that migrate uses, including:
//   - GET /api/v2/file/{uuid}/ - retrieve package details
//   - POST /api/v2/file/{uuid}/move/ - initiate a package move
//   - GET /api/v2/location/{uuid}/ - retrieve location details
//   - POST /_internal/replicate - create package replicas
//
// ## Integration with testscript
//
// The package provides two testscript commands:
//   - ssmock start -config <file> (starts the mock server)
//   - ssmock snapshot (captures current state for assertions)
//
// Migrate uses ssmock- you can find some examples under `/testdata/*.txtar`.
// The mock is configured using TOML files that define locations and initial
// package state. See [Config] for the schema. Example:
//
// The [Server.Snapshot] method provides a thread-safe, immutable view of the
// current server state. This is particularly useful for test assertions, as it
// allows you to inspect package locations, statuses, and location usage without
// worrying about concurrent modifications.
package ssmock
