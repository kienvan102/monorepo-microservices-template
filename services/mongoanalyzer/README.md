# mongoanalyzer

The service holding the analysis scripts that [`db-analyzer`](../../cmd/db-analyzer/README.md) runs. This README is for people writing or editing scripts. To run an analysis and read its results, see the `db-analyzer` README.

Scripts live in `scripts/`, are written in JavaScript, and run inside `mongosh`. They only read data — they never create/drop indexes or modify data.

## Common scripts vs. dedicated scripts

**Common scripts** only use information every collection has, like its size or index list. The collection name comes from configuration, never hardcoded into the script — so the same script works for any collection. Common scripts live in `scripts/common/`. For example, `common/indexes` lists the indexes of whichever collection is currently configured, for any collection.

**Dedicated scripts** use a specific collection's field names, filter conditions, or queries, so they only make sense for that one collection. For example, counting orders by their `status` field only makes sense for the `orders` collection. Dedicated scripts live in `scripts/collections/<collection name>/`, and only run when the currently configured collection matches that directory's name.

## Adding a script

Example: add a dedicated script `status_counts` for the `orders` collection, counting orders by status.

**1. Write the script** at `scripts/collections/orders/status_counts.js`. The filename must start with a letter and contain only letters, digits, `_`, or `-`.

```javascript
(() => {
  const options = globalThis.MONGO_ANALYSIS_OPTIONS || {};
  const orders = db.getCollection(options.collection);
  const count = (status) => orders.countDocuments({ status }, { maxTimeMS: options.queryTimeoutMS });
  const result = { pending: count('pending'), shipped: count('shipped') };
  print(`ANALYZER_RESULT_JSON:${EJSON.stringify(result)}`);
})();
```

The script runs through `mongosh`, in a session already connected to the database named by `MONGO_DATABASE` (the `db` variable). So it's written exactly the way you'd write it when running it by hand in `mongosh`.

The script receives configuration through the `MONGO_ANALYSIS_OPTIONS` variable. `db-analyzer` populates it automatically from its own configuration:

| In the script | Comes from configuration value |
| --- | --- |
| `options.collection` | `MONGO_COLLECTION` |
| `options.queryTimeoutMS` | `QUERY_TIMEOUT`, converted to milliseconds |
| `options.sampleSize` | `SAMPLE_SIZE` |
| `options.explainLimit` | `EXPLAIN_LIMIT` |

Everything the script prints gets saved into the result file:

- A line starting with `ANALYZER_RESULT_JSON:`, followed by a JSON object or array: that value is written into the `output_json` field.
- A line starting with `ANALYZER_PROGRESS_JSON:`, followed by JSON: the result of one step. If the script gets interrupted partway through, these lines are kept in the `partial_results` field. A script with multiple steps should print one of these after each step.
- Everything else printed (`print`, `printjson`, ...) goes into the `stdout` field.

The whole script runs for at most `SCRIPT_TIMEOUT`, and gets killed past that. To bound an individual query, pass `maxTimeMS: options.queryTimeoutMS` into that query. For `explain`, set `maxTimeMS` on the `explain` call itself. A query that times out throws and stops the script, so wrap independent steps in `try/catch` if you want later steps to keep running.

**2. Register the script in a plan.** A plan is a JSON file listing the scripts that `inspect`/`collect` will run, in the exact order given. There are two kinds of plan:

- `scripts/plan.json`: the list of common scripts, under the `inspect` key, each entry the script's full name. Both `inspect` and `collect` run this list.

  ```json
  { "inspect": ["common/collection_stats", "common/indexes"] }
  ```

- `scripts/collections/<collection name>/plan.json`: the collection's list of dedicated scripts, under the `default` key, each entry just the filename, without `.js` and without a path. `collect` runs this list after the common scripts.

For this example, create `scripts/collections/orders/plan.json`. If the file already exists, just add `"status_counts"` to the list.

```json
{ "default": ["status_counts"] }
```

**3. Run it.** In `db-analyzer`'s configuration, set the collection to `orders` (`mongo.collection` in `config.yaml`, or `MONGO_COLLECTION` in `.env`), then:

```bash
make db-analyzer-collect                                       # common scripts, then status_counts
make db-analyzer-run SCRIPT=collections/orders/status_counts   # run only status_counts
```

The result is written to `collections__orders__status_counts.json`. A script that hasn't been registered in a plan yet can still be run with `make db-analyzer-run`. `make` commands rebuild the binary automatically, so a new script takes effect immediately.
