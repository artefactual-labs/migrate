# Migrate

**Migrate** is a command-line tool for orchestrating large-scale **Archivematica
AIP migrations**. It automates the process of **moving and replicating AIPs**
between Archivematica Storage Service locations, e.g. migrating AIPs from
on-premises storage into DuraCloud while keeping a local replica set.

## Why use this tool?

Archivematica’s Storage Service already supports moving and replicating AIPs,
but running those operations at **scale** (tens of thousands of AIPs) is:

- **Slow and error-prone** if done manually
- **Hard to resume** if interrupted
- **Lacking visibility** into what has succeeded or failed

**Migrate** solves these problems by adding a durable orchestration layer:

- **Move and replicate AIPs** between Storage Service locations
- **Batch processing** from a simple UUID list (`input.txt`)
- **Reliable orchestration** with [Temporal] so workflows survive crashes and
  restarts
- **State tracking** in a local SQLite database (`migrate.db`)
- **Parallel execution** across multiple workers
- **CSV reports** (`move-report.csv`, `replication-report.csv`) for QA,
  auditing, and compliance

In short: give it a list of AIPs, and Migrate will orchestrate every move and
replication reliably until the job is done.

## Why is this not part of Storage Service?

Because Storage Service was never designed to be a workflow engine. It can move
and replicate AIPs, but it has no durable queue, no orchestration layer, and no
good way to resume or audit thousands of operations. It works for one-off jobs,
but at scale it becomes brittle.

Migrate fills that gap: it wraps Storage Service operations in durable Temporal
workflows, adding the missing layer of **scale, reliability, and reporting**
that Storage Service can't provide on its own. Over time, we expect new
approaches to emerge for large-scale digital preservation workflows, but today
Migrate is the practical tool to get the job done.

## How it works

Migrate separates **command submission** from **execution**:

- The **client** (`migrate replicate` / `migrate move`) submits workflows to
  Temporal.
- The **worker** (`migrate worker`) picks up tasks and executes them: talks to
  the Storage Service API, requests move or replication operations, checks
  fixity, and updates the database.
- The **SQLite DB** (`migrate.db`) keeps track of every AIP's state.
- You can generate **reports** (`migrate export`) at any time.

The following diagram illustrates the basic architecture:

```mermaid
flowchart TD
    A[Client: migrate CLI] -->|Submit workflows| B[Temporal Server]
    B --> C[Worker: migrate worker]
    C --> D[Archivematica Storage Service]
    D -->|Move/Replicate| E1[Storage Location A]
    D -->|Move/Replicate| E2[Storage Location B]
    D -->|Move/Replicate| E3[Storage Location C]
    C --> F[(SQLite DB: migrate.db)]
    F --> G[CSV Reports]
```

## Prerequisites

- Access to an **Archivematica Storage Service** instance with valid API
  credentials.

- A running **Temporal Server**:

  1. **Production-ready deployment (recommended):**

     Connect to a server already maintained by your organization, use [Temporal
     Cloud], or deploy your own cluster by following the [production deployment
     guide]. Be aware that operating and maintaining your own cluster adds
     substantial complexity to your setup.

  2. **Local development (quick start):**

     Start a lightweight development server with the [Temporal CLI]:

         temporal server start-dev --db-filename ./temporal.db

- Install Migrate: prebuilt packages are available from the [releases page].

## Configuration

### 1. Create configuration file

Create a `config.json` file in the project root with the **actively used settings**:

```json
{
  "ss_url": "http://your-storage-service:8000",
  "ss_user_name": "your_username",
  "ss_api_key": "your_api_key",
  "move_location_uuid": "uuid-of-move-destination",
  "location_uuid": "uuid-of-source-location",
  "ss_manage_path": "/usr/share/archivematica/storage-service/manage.py",
  "python_path": "/usr/bin/python3",
  "docker": false,
  "ss_container_name": "archivematica-storage-service",
  "replication_locations": [
    {
      "uuid": "location-1-uuid",
      "name": "Backup Location 1"
    },
    {
      "uuid": "location-2-uuid",
      "name": "Backup Location 2"
    }
  ],
  "environment": {
    "django_settings_module": "storage_service.settings.production",
    "django_secret_key": "your-secret-key",
    "django_allowed_hosts": "*",
    "ss_gunicorn_bind": "0.0.0.0:8000",
    "email_host": "localhost",
    "ss_audit_log_middleware": "false",
    "ss_db_url": "sqlite:///var/lib/archivematica/storage-service/storage_service.db",
    "email_use_tls": "false",
    "ss_prometheus_enabled": "false",
    "default_from_email": "noreply@example.com",
    "time_zone": "UTC",
    "ss_gunicorn_workers": "2",
    "requests_ca_bundle": ""
  },
  "dashboard": {
    "manage_path": "/usr/share/archivematica/dashboard/manage.py",
    "python_path": "/usr/bin/python3",
    "lang": "en_US.UTF-8",
    "django_settings_module": "settings.production",
    "django_allowed_hosts": "*",
    "django_secret_key": "your-dashboard-secret-key",
    "gunicorn_bind": "0.0.0.0:8002",
    "elastic_search_url": "http://your-elasticsearch:9200",
    "ss_client_quick_timeout": "5"
  }
}
```

### 2. Create input file

Create an `input.txt` file containing the UUIDs of AIPs you want to process
(one UUID per line):

```console
12345678-1234-1234-1234-123456789abc
87654321-4321-4321-4321-cba987654321
abcdef01-2345-6789-abcd-ef0123456789
```

If you need to trim an existing UUID list before loading it, a small helper
command lives in `cmd/list-filter`. See its README for usage details.

### 3. Load input file

```bash
./migrate load-input
```

This validates the UUIDs in `input.txt` and initializes them in the database.

### 4. Start worker process

```bash
./migrate worker
```

This starts a worker process that handles Temporal workflows. Keep this
running in a separate terminal.

### 5. Move or replicate AIPs

At this point, you can either `replicate` or `move` AIPs.

The following command starts the replication process for AIPs to the configured
replication locations:

    migrate replicate

On the other hand, to move AIPs from source to destination, run:

    migrate move

### 6. Export results

Generate CSV reports for move or replication workflows:

    migrate export move
    migrate export replicate

Each command writes the corresponding report (`move-report.csv` or
`replication-report.csv`) with the latest status for every AIP.

[Temporal]: https://temporal.io
[Temporal CLI]: https://docs.temporal.io/cli/setup-cli
[Temporal Cloud]: https://temporal.io/cloud
[production deployment guide]: https://docs.temporal.io/production-deployment
[releases page]: https://github.com/artefactual-labs/migrate/releases
