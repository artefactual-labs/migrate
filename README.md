# Archivematica AIP Migration Tool

A Go-based tool for migrating, replicating, and moving Archivematica AIPs
(Archival Information Packages) between storage locations using Temporal
workflows.

## What This Tool Does

This tool helps you:

- **Migrate AIPs** between different storage locations in Archivematica
- **Replicate AIPs** to create copies in multiple locations for redundancy
- **Move AIPs** from one location to another
- **Track migration progress** using a SQLite database
- **Process AIPs in batch** using UUID lists
- **Generate reports** on migration status

## Prerequisites

Before you can use this tool, you need:

1. **Go 1.23 or later** installed on your system
   - Download from: <https://golang.org/dl/>
   - Verify installation: `go version`

2. **musl-gcc** (for static linking with SQLite)
   - Ubuntu/Debian: `sudo apt-get install musl-tools musl-dev`
   - CentOS/RHEL: `sudo yum install musl-gcc`
   - macOS: `brew install musl-cross`

3. **Temporal Server** (workflow orchestration)
   - Follow installation guide: <https://docs.temporal.io/cli/setup-cli>
   - Or run with Docker: `temporal server start-dev`

4. **Access to Archivematica Storage Service**
   - Storage Service API credentials
   - Network access to your Archivematica instances

## Installation

### 1. Clone the Repository

```bash
git clone https://gitlab.artefactual.com/dcosme/migrate.git
cd migrate
```

### 2. Install Go Dependencies

```bash
go mod download
```

### 3. Build the Tool

```bash
./scripts/build.sh
```

This creates a statically-linked binary called `migrate` in the current directory.

## Configuration

### 1. Create Configuration File

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
  "use_temporal": true,
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

**Note:** This shows only the settings that are actively used by the current
implementation. For a complete list of possible settings (including many that
have no effect), see the [Additional Configuration Options](#additional-configuration-options)
section below.

### 2. Create Input File

Create an `input.txt` file containing the UUIDs of AIPs you want to process
(one UUID per line):

```console
12345678-1234-1234-1234-123456789abc
87654321-4321-4321-4321-cba987654321
abcdef01-2345-6789-abcd-ef0123456789
```

## Usage

### Start Temporal Server

First, start your Temporal server:

```bash
temporal server start-dev --db-filename path/to/local-persistent-store
```

Always specify `--db-filename`, otherwise Temporal will use an in-memory
database and all data will be lost when the process stops.

### Running the Tool

The tool supports several commands:

#### 1. Load Input and Initialize Database

```bash
./migrate load-input
```

This validates the UUIDs in `input.txt` and initializes them in the database.

#### 2. Start Worker Process

```bash
./migrate worker
```

This starts a worker process that handles Temporal workflows. Keep this
running in a separate terminal.

#### 3. Replicate AIPs

```bash
./migrate replicate
```

This replicates AIPs to the configured replication locations.

#### 4. Move AIPs

```bash
./migrate move
```

This moves AIPs from source to destination location.

#### 5. Export Results

```bash
./migrate export
```

This generates a CSV report (`report.csv`) showing the status of all processed AIPs.

#### 6. Pause Operations

```bash
./migrate pause
```

This pauses ongoing operations (useful for stopping batch processes). For
detailed information about pausing and resuming operations, see the
[Pause and Resume Operations](#pause-and-resume-operations) section.

### Typical Workflow

1. **Prepare your environment:**

   ```bash
   # Start Temporal server
   temporal server start-dev
   ```

2. **Set up the tool:**

   ```bash
   # Create config.json with your settings
   # Create input.txt with AIP UUIDs
   ./migrate load-input
   ```

3. **Start the worker (in separate terminal):**

   ```bash
   ./migrate worker
   ```

4. **Run migration operations:**

   ```bash
   # For replication
   ./migrate replicate
   
   # Or for moving
   ./migrate move
   ```

5. **Check results:**

   ```bash
   ./migrate export
   # Check report.csv for results
   ```

## Troubleshooting

### Common Issues

**"missing command" error:**

- Make sure you're providing a command: `./migrate replicate` (not just `./migrate`)

**"cannot run this workflow without enabling temporal" error:**

- Ensure `use_temporal: true` in your `config.json`
- Check that Temporal server is running

**Database errors:**

- The tool creates `migrate.db` automatically
- Delete `migrate.db` to reset and start fresh

**Configuration errors:**

- Verify all UUIDs in `config.json` are valid
- Check that URLs are reachable from your system
- Ensure API credentials are correct

### Getting Help

- Check the logs for detailed error messages
- Verify your Archivematica Storage Service is accessible
- Ensure all file paths in the configuration exist
- Make sure you have sufficient permissions for staging/local paths

## File Structure

- `config.json` - Configuration file (you create this)
- `input.txt` - List of AIP UUIDs to process (you create this)
- `migrate.db` - SQLite database (created automatically)
- `report.csv` - Export results (created by export command)
- `migrate` - The compiled binary (created by build script)

## Notes

- This tool uses Temporal workflows for reliability and observability
- Progress is tracked in a local SQLite database
- The tool validates UUIDs before processing
- Operations can be paused and resumed
- All file operations use staging areas to prevent data loss

## Additional Configuration Options

The configuration file supports many additional settings beyond those shown in
the main example. While these settings are **structurally valid** and can be
included in your configuration, **most have no effect** on the current
implementation:

### Legacy/Deprecated Settings

- **`replication_location_uuid`** - Superseded by the `replication_locations`
  array which supports multiple destinations
- **`elastic_search_url`** - Defined but not used in active functionality
- **`staging_path`** - Defined but not used in active functionality  
- **`local_copy_path`** - Defined but not used in active functionality

### Feature Toggle Settings (Currently Inactive)

- **`check_fixity`** - Can be set but functionality exists only in
  commented-out code
- **`move`** - Can be set but not used by current workflow logic
- **`clean`** - Can be set but functionality exists only in commented-out code  
- **`re_index`** - Can be set but not actively implemented
- **`Replicate`** - Can be set but not used by current workflow logic

### Additional Dashboard Settings

You can include additional dashboard configuration options beyond those shown in
the main example. Some are planned but not yet implemented - these inactive
settings exist as legacy code from earlier versions, placeholders for future
functionality, or for environment compatibility but are not actively used:

```json
{
  "dashboard": {
    "elastic_search_timeout": "30",
    "audit_log_middleware": "false", 
    "prometheus_enabled": "false",
    "email_host": "localhost",
    "email_default_from": "noreply@example.com",
    "time_zone": "UTC",
    "search_enabled": "true",
    "client_host": "localhost",
    "client_database": "MCP",
    "client_user": "archivematica",
    "client_password": "demo", 
    "requests_ca_bundle": ""
  }
}
```

## Pause and Resume Operations

The migration tool supports pausing and resuming batch operations to provide
flexibility during long-running migration processes.

### How Pause Works

To pause ongoing operations, run:

```bash
./migrate pause
```

**Important notes about pausing:**

- The pause command sets a flag that prevents new AIPs from being processed
- It only takes effect **between** AIP processing iterations, not during the
  processing of an individual AIP
- Any AIP currently being processed will complete before the operation stops
- The tool will stop processing additional AIPs from the input list

### How to Resume Operations

**There is no explicit resume command.** To resume operations after pausing:

1. **Simply run the same command again:**

   ```bash
   # If you were running replication
   ./migrate replicate
   
   # If you were running moves  
   ./migrate move
   ```

2. **The tool automatically resumes where it left off** by:
   - Checking the status of each AIP in the SQLite database (`migrate.db`)
   - Skipping AIPs that have already been processed successfully
   - Only processing AIPs that are still pending or failed

### Resume Behavior

When you restart a command, the tool will:

- **For replication:** Skip AIPs with status `replicated`
- **For moves:** Skip AIPs with status `moved`
- **For all operations:** Skip AIPs with status `not-found` (AIPs that don't exist)
- Continue processing from the next unprocessed AIP in your `input.txt` file

### Example Workflow

```bash
# Start replication
./migrate replicate

# ... processing 50 AIPs ...
# Press Ctrl+C or run in another terminal:
./migrate pause

# Later, resume where you left off
./migrate replicate
# Tool automatically skips the 50 already-processed AIPs and continues
```

This design ensures that no work is duplicated and you can safely resume
operations at any time without losing progress.
