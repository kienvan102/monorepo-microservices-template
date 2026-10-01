# db-analyzer

A command-line tool that analyzes a MongoDB collection to support index design. Once it finishes, you get a directory of JSON files: each one answers a single question about the collection — for example, which indexes it currently has, or whether the application's queries actually use the right index. The tool is read-only: it never creates/drops indexes or modifies data.

## First run

Run every command from the repo root.

**1. Install:** Go 1.27+, Make, and [`mongosh`](https://www.mongodb.com/docs/mongodb-shell/install/) available on the command line. The analyses themselves run through `mongosh`.

**2. Pick the database to analyze.** Open `cmd/db-analyzer/config.yaml` and change `mongo.database` and `mongo.collection` to your own database and collection. Don't put a connection URI with a password in this file — put it in `.env` instead:

```bash
cp cmd/db-analyzer/.env.example cmd/db-analyzer/.env
# edit the MONGO_URI line in cmd/db-analyzer/.env
```

**3. Check the connection:**

```bash
make db-analyzer-check-connection
```

Seeing `connected to database` means the connection works and the collection exists.

**4. Run an analysis:**

```bash
make db-analyzer-inspect    # common analysis: works with any collection
make db-analyzer-collect    # common analysis + this collection's dedicated analyses
```

`collect` only works for a collection that already has a dedicated analysis set — currently just `threads_posts`. For any other collection, `collect` fails; use `inspect` instead. On a large collection, `collect` can take a few minutes, since it counts and runs `explain` against real data — run it during low traffic if that's a production database.

**5. Read the results.** The last log line tells you the output directory:

```text
INF run complete failures=0 output_dir=output/testing/20260925T090605.899204840Z scripts=5
```

Inside that directory:

- **`manifest.json`:** the list of analyses that ran, each with a `status` of `ok` or `error` plus a reason. Open this file first.
- **One JSON file per analysis**, with its result in the `output_json` field. See the table below for what each file answers.

## What each result file tells you

| File | Answers | Where to look in `output_json` |
| --- | --- | --- |
| `common__collection_stats.json` | How big is the collection? | `estimated_document_count`, `data_size_bytes`, `total_index_size_bytes` |
| `common__indexes.json` | What indexes does the collection currently have? | `indexes`: name and fields of each index |
| `collections__<collection>__schema_sample.json` | What does the real data actually look like? | `field_types`: how many records are missing each field, and its type (based on a sample) |
| `collections__<collection>__filter_selectivity.json` | How many records does each of the app's filter conditions match? | `results`: `count` per condition; the smaller it is relative to the total, the more that condition belongs in an index |
| `collections__<collection>__workload_explain.json` | Which index does the app's query use, and is it effective? | `results`: `indexes_used`, and inside `explain.executionStats`, compare `totalDocsExamined` to `nReturned` — reading far more than is returned means the query needs a better index |

The first two files exist for both `inspect` and `collect`. The last three exist only for `collect`.

An analysis with `status: ok` can still have failures in individual measurements inside it — for example, one filter's count exceeding the timeout — recorded in that measurement's own `error` field.

## Commands

| Command | What it does |
| --- | --- |
| `make db-analyzer-check-connection` | Connects and confirms the collection exists. |
| `make db-analyzer-list` | Prints the names of the available analyses. |
| `make db-analyzer-inspect` | Runs the common analyses. |
| `make db-analyzer-collect` | Runs the common analyses, then the collection's dedicated ones. |
| `make db-analyzer-run SCRIPT=<name>` | Runs exactly one analysis, by name from `list`. |

To use a different configuration file, add `CONFIG_FILE=<path>` or `ENV_FILE=<path>` to the `make` command.

Exit codes: `0` if every analysis is `ok`; `1` if any analysis fails or there's a configuration/connection error; `130` if stopped with Ctrl-C before finishing, `143` if stopped with SIGTERM (e.g. `docker stop`). When one analysis fails, the results of the others are still written.

## Configuration

How configuration sources override each other (`config.yaml`, `.env`, environment variables): see the [root README](../../README.md#configuration).

| Key in `config.yaml` | Variable (`.env`) | Code default | Meaning |
| --- | --- | --- | --- |
| `mongo.uri` | `MONGO_URI` | `mongodb://localhost:27017` | Connection URI. A password with special characters (e.g. `$`) must be percent-encoded. |
| `mongo.database` | `MONGO_DATABASE` | `test` | Database to analyze. |
| `mongo.collection` | `MONGO_COLLECTION` | `threads_posts` | Collection to analyze; determines which dedicated analysis set `collect` runs. |
| `outputDir` | `OUTPUT_DIR` | `output` | Where results are written. A relative path is resolved from the directory containing `config.yaml`; `config.yaml` sets `../../output/testing`, i.e. `output/testing/` at the repo root. |
| `queryTimeout` | `QUERY_TIMEOUT` | `20s` | Maximum time for the connection check and for each query inside the dedicated analyses. |
| `scriptTimeout` | `SCRIPT_TIMEOUT` | `3m` | Maximum time for a single analysis. |
| `sampleSize` | `SAMPLE_SIZE` | `500` | Number of records sampled for `schema_sample`. |
| `explainLimit` | `EXPLAIN_LIMIT` | `100` | Maximum number of results per query when running `explain`. |
| `appEnv` | `APP_ENV` | `dev` | `dev`/`testing`: human-readable logs with debug output. `staging`/`production`: JSON logs. |

## Calling the binary directly

`make` builds `bin/db-analyzer`. Call it directly when needed:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-action` | `inspect` | `check`, `list`, `inspect`, `collect`, `run`. |
| `-script` | | Analysis name, for `-action run`. |
| `-config` | `config.yaml` | YAML configuration file. Set to empty (`-config=`) to disable. |
| `-env-file` | `.env` | `.env` file. Skipped if it doesn't exist; set to empty (`-env-file=`) to disable reading it. |
| `-scripts-dir` | | Read scripts from a directory on disk instead of the copy embedded in the binary — use this while editing scripts: `-scripts-dir services/mongoanalyzer/scripts`. |

## Running with Docker

No need to install Go or `mongosh` — the image already has them:

```bash
make db-analyzer-image
docker run --rm --env-file cmd/db-analyzer/.env db-analyzer -action check
docker run --rm --env-file cmd/db-analyzer/.env -v "$PWD/out:/out" db-analyzer -action collect
```

The image uses the deployment's `config.yaml`. Variables passed via `--env-file`/`-e` override it. Results are written to `/out` inside the container, so mount a directory there, and that directory must be writable by the container's user (uid 65532).

> The image has never been built, since the current development machine doesn't have Docker permissions.

## Adding a new analysis

See the [mongoanalyzer README](../../services/mongoanalyzer/README.md).
