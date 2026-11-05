# mongoanalyzer

The scripts that [`db-analyzer`](../../cmd/db-analyzer/README.md) runs to analyze a MongoDB collection for index design. This page is for adding scripts and queries and for running them. Installing the tools, connecting to the database and the meaning of each setting are in the `db-analyzer` README ("First run").

Scripts are JavaScript files that run inside `mongosh` and save what they print as JSON. They only read data, with one exception: `threads_posts/monitoring_candidate_ablation` temporarily hides indexes (`collMod`) to compare query plans, and restores them when it finishes. Run that one only when you mean to.

What do you want to do?

- [Analyze a collection](#analyze-a-collection)
- [Explain a query](#explain-a-query)
- [Add a script](#add-a-script)
- [Run some of the scripts, or one](#run-some-of-the-scripts-or-one)
- [Read the result](#read-the-result)
- [When something goes wrong](#when-something-goes-wrong)
- [Reference](#reference)

## Analyze a collection

```bash
MONGO_COLLECTION=orders make db-analyzer-inspect    # works on any collection
MONGO_COLLECTION=orders make db-analyzer-collect    # inspect, then this collection's own analysis
```

`inspect` runs the **common scripts**, which need nothing but the collection itself (its size and its index list). It works on every collection.

`collect` runs the common scripts and then the collection's own **plan**: the scripts listed in `scripts/collections/<collection>/plan.json`. It only works for a collection that has such a folder and file. Today those are `threads_posts`, `threads_replies` and `identity_crawling_7`. For another collection it stops with `no analysis plan for collection`; use `inspect`, or [add a script](#add-a-script) to create the plan.

`make db-analyzer-list` prints every script the tool knows.

## Explain a query

Use this to see how a query runs: which index it uses, how many keys and documents it reads, and which of your indexes would serve it best. A collection that supports this (`threads_posts`, `threads_replies`) keeps each query in its own file in a `workloads/` folder.

**1. Write the query** as a file `scripts/collections/<collection>/workloads/<name>.js`. The file is one JavaScript function, with no `;` after it. It returns `{ filter, sort }` for a find, or `{ pipeline }` for an aggregation:

```javascript
({ now, ago }) => ({
  description: "open orders from the last 30 days",
  filter: {
    status: "open",
    created_at: { $gte: ago({ days: 30 }), $lte: now },
  },
  sort: { created_at: -1 },
})
```

The function is given these helpers: `now`, `nowSeconds`, `ago({ days, hours, minutes })` for a date that long before now, `agoSeconds({ ... })` for the same in epoch seconds, and `nextMinute(45)` for the next time the clock reads minute 45. Leave `sort` out if the real query has none.

Prefer plain ranges (`$gte`, `$lte`) over `$expr` with `$$NOW`: the planner can use an index for them. A plain range does not match a record whose field is null or missing, so its count can be lower than the `$expr` version.

**2. List it** in `scripts/collections/<collection>/plan.json`, in the `items` of the `workloads` group, so the explain scripts include it:

```json
"items": ["comment_loader", "your_query_name"]
```

Write the file name only, without `.js`. A query that is not listed can still be run by name (next step).

**3. Run it.**

```bash
MONGO_COLLECTION=orders make db-analyzer-run SCRIPT=collections/orders/workloads/your_query_name   # only this query
MONGO_COLLECTION=orders make db-analyzer-run SCRIPT=collections/orders/workload_explain            # every listed query
MONGO_COLLECTION=orders make db-analyzer-run SCRIPT=collections/orders/index_candidates            # every listed query, once per existing index (slow)
```

Running a single query always uses `workload_explain`. `index_candidates` has no single-query form; it runs the listed queries.

**4. Read the result:** see [Read the result](#read-the-result).

**A collection without a `workloads` folder yet.** The two explain scripts are ordinary scripts that belong to one collection. To use them on a new one:

1. Copy `workload_explain.js` and `index_candidates.js` from `scripts/collections/threads_replies/` into `scripts/collections/<collection>/`.
2. In both copies, replace the collection name (`threads_replies`) with yours. Search for it; it appears in the guard at the top.
3. Write your query file as above.
4. Create or edit `scripts/collections/<collection>/plan.json`:

```json
{
  "default": ["workload_explain"],
  "groups": {
    "workloads": {
      "engines": ["workload_explain", "index_candidates"],
      "global": "MONGO_ANALYSIS_WORKLOADS",
      "items": ["your_query_name"]
    }
  }
}
```

The explain scripts are copied, not shared, so a fix to one copy has to be made in the others.

## Add a script

Example: a script `status_counts` for the `orders` collection that counts orders by status. Write it, register it if you want it in the standard analysis, then run it.

**1. Write it** at `scripts/collections/orders/status_counts.js`. The name starts with a letter and uses only letters, digits, `_` or `-`.

```javascript
(() => {
  const options = globalThis.MONGO_ANALYSIS_OPTIONS || {};
  const orders = db.getCollection(options.collection);
  const count = (status) => orders.countDocuments({ status }, { maxTimeMS: options.queryTimeoutMS });
  const result = { pending: count('pending'), shipped: count('shipped') };
  print(`ANALYZER_RESULT_JSON:${EJSON.stringify(result)}`);
})();
```

It runs through `mongosh` in a session already connected to the database (the `db` variable), so write it as you would by hand in `mongosh`. What it receives and must print is in [What a script receives and prints](#what-a-script-receives-and-prints).

A script that works for any collection goes in `scripts/common/` instead and must not hardcode a collection name.

**2. Register it** only if `collect` should run it every time. Otherwise skip this step: the script is **on-demand** and you run it by name.

Registering means adding the script's name to a list in a `plan.json`. For `orders` that file is `scripts/collections/orders/plan.json`.

- If the collection has registered scripts, the file is there and its `default` list holds them. For example:

  ```json
  { "default": ["schema_sample"] }
  ```

  Add a comma and your script's name after the last name, inside the brackets, and save:

  ```json
  { "default": ["schema_sample", "status_counts"] }
  ```

- If this is the first script of a new collection, there is no `plan.json` in its folder. Create it with just:

  ```json
  { "default": ["status_counts"] }
  ```

Write the file name only: no `.js`, no folder. Put commas between names and none after the last. Comments are not allowed, because the file is plain JSON.

For a common script, add `"common/<name>"` to the `inspect` list in `scripts/plan.json`.

**3. Run it:**

```bash
MONGO_COLLECTION=orders make db-analyzer-collect                                       # common scripts, then status_counts
MONGO_COLLECTION=orders make db-analyzer-run SCRIPT=collections/orders/status_counts   # only status_counts
```

`make` rebuilds the binary first, so a new script takes effect at once. `make db-analyzer-list` should show its name.

## Run some of the scripts, or one

- **Everything in the plan:** `make db-analyzer-collect`.
- **One script:** `make db-analyzer-run SCRIPT=<name>`, with a name from `make db-analyzer-list`.
- **Several scripts:** there is no way to name more than one in a command. Run them one at a time, or put exactly the ones you want in the plan.
- **A few queries out of many:** run each by name, or leave only those in the `items` list and run `workload_explain`.

Slow scripts need longer limits. `QUERY_TIMEOUT` bounds each query and `SCRIPT_TIMEOUT` bounds a whole script, which is killed past it:

```bash
QUERY_TIMEOUT=120s SCRIPT_TIMEOUT=30m MONGO_COLLECTION=orders make db-analyzer-run SCRIPT=collections/orders/index_candidates
```

## Read the result

Each run creates a new folder under the output directory, and the last log line prints its path. Open `manifest.json` first: it lists every script that ran, with `ok` or `error`. Each script then has its own file, named after it: `collections__orders__status_counts.json`, or `collections__orders__workloads__your_query_name.json` for a single query.

In each file, `output_json` is what the script printed as its result. If the script was cut off, `partial_results` holds the steps that finished. Anything else it printed is in `stdout`. What the files from the standard scripts answer is in the `db-analyzer` README.

For the explain scripts:

- **`workload_explain`:** each query appears twice, as `limited` (capped like the real loader) and `full` (no cap, which reads every matching record). Under `full`, compare `totalKeysExamined` and `totalDocsExamined` with `nReturned`: reading far more than is returned means the query needs a better index. `usedIndexes` and `winningPlan` show which index and plan were used. A query that failed has `error` instead.
- **`index_candidates`:** for each query, `plannerIndexes` is the index the database chose, `best` is the existing index that read least, and `candidates` lists every index by what it read. Under `indexes`, an index with `neverChosenNorBest: true` was not the best for any listed query. That does not prove it can be dropped: it may be the fallback when another index goes.

## When something goes wrong

| You see | Cause | What to do |
| --- | --- | --- |
| `unknown script "..."` or `script "..." not found` | The name has a typo, the file is not in that folder, or (for a query) its group is not in `plan.json` | `make db-analyzer-list` prints the names the tool knows. Use one of those |
| `script "..." is for threads_posts; .env selects orders` | The script belongs to a different collection than the one you are running | Set `MONGO_COLLECTION` to the script's collection |
| An error that starts with `collections/<collection>/plan.json:` and mentions an invalid character | `plan.json` is not valid JSON: usually a comment, or a comma after the last name | Remove the comment or the comma |
| `group "...": engine "..." not found` or `item "..." not found` | A name in the group entry has no file | Fix the name, or add the file. Names are file names without `.js` |
| `no analysis plan for collection` | `collect` on a collection with no `plan.json` | Use `inspect`, or add a script to create the plan |

## Reference

### What a script receives and prints

The script receives configuration in the `MONGO_ANALYSIS_OPTIONS` variable, which `db-analyzer` fills from its own configuration:

| In the script | Comes from configuration value |
| --- | --- |
| `options.collection` | `MONGO_COLLECTION` |
| `options.queryTimeoutMS` | `QUERY_TIMEOUT`, converted to milliseconds |
| `options.sampleSize` | `SAMPLE_SIZE` |
| `options.explainLimit` | `EXPLAIN_LIMIT` |

What the script prints is saved into its result file:

- A line starting with `ANALYZER_RESULT_JSON:` followed by a JSON object or array: written into `output_json`.
- A line starting with `ANALYZER_PROGRESS_JSON:` followed by JSON: the result of one step. If the script is interrupted, these lines are kept in `partial_results`. A script with several steps should print one after each step.
- Everything else printed (`print`, `printjson`, ...) goes into `stdout`.

To bound an individual query, pass `maxTimeMS: options.queryTimeoutMS` into it; for `explain`, set `maxTimeMS` on the `explain` call. A query that times out throws and stops the script, so wrap independent steps in `try/catch` if later steps should still run.

### Where things live

```text
scripts/
├── plan.json                        the common plan: the common scripts that inspect and collect run
├── common/
│   └── <script>.js                  a script that works on any collection
└── collections/
    └── <collection>/                everything for one collection
        ├── plan.json                that collection's plan: its scripts, in order, and its groups
        ├── <script>.js              a script for this collection
        └── <group>/<item>.js        one of many small files run by a single script
```

A dedicated script only runs when the configured collection matches its folder name. In each collection, the names under `default` in `plan.json` are the **plan scripts** that `collect` runs every time. Every other `.js` file in the folder is an **on-demand script**, run by name. Today:

| Collection | Plan scripts | On-demand scripts |
| --- | --- | --- |
| `threads_posts` | `schema_sample`, `filter_selectivity`, `workload_explain` | `index_candidates`, `index_usage`, `schema_profile`, `collection_baseline`, `monitoring_candidate_ablation` |
| `threads_replies` | `collection_baseline`, `schema_sample`, `filter_selectivity`, `candidate_validation`, `workload_explain` | `index_candidates` |
| `identity_crawling_7` | `workload_explain` | `collection_profile`, `identity_crawling_7`, `monthly_profile_plan` |
| `identity_crawling_10` | none, it has no `plan.json` | `workload_explain` |

### Which plan entry a new file needs

| You wrote | Add its name to | In this file |
| --- | --- | --- |
| A script that works for any collection | `inspect` (as `common/<name>`) | `scripts/plan.json` |
| A script for one collection | `default` | `scripts/collections/<collection>/plan.json` |
| One more query for the explain scripts | that group's `items` | `scripts/collections/<collection>/plan.json`, under `groups` |
| A script that processes all of a group's items | that group's `engines`, and also `default` if `collect` should run it | `scripts/collections/<collection>/plan.json` |
| A new family of similar things | a new entry under `groups`, with `engines`, `global` and `items` | `scripts/collections/<collection>/plan.json` |

If you are unsure, you wrote one script for one collection, and the entry is `default`.

### Groups: many similar things run by one script

The explain feature is a **group**. Use one when you would otherwise copy a script many times, changing only the query inside. A group is a folder of small **item** files plus a script, the **engine**, written once, that runs all the items and writes one result file covering them. Run by name, an item is run by the group's first engine and gets a result file of its own.

A group is declared in the collection's `plan.json`, next to `default`:

```json
{
  "default": ["explain_queries"],
  "groups": {
    "queries": {
      "engines": ["explain_queries"],
      "global": "ORDER_QUERIES",
      "items": ["open_orders"]
    }
  }
}
```

- **The key (`queries`)** is the group's name and must match its folder, `scripts/collections/orders/queries/`. Choose it once.
- **`items`** are the files in that folder that an engine runs on its own run, in order. Add a name for one more item; remove one to stop running it, and its file stays.
- **`engines`** are the scripts that process the items, by file name without `.js`. Add one for one more script that should process the same items. The first engine runs a single item named on its own.
- **`global`** is the JS variable the engine reads its items from. Choose it once and use the same name in the engine. It must be a valid variable name, and not `MONGO_ANALYSIS_OPTIONS`.

The engine is also listed in `default`, like any plan script, so `collect` runs it. It is listed under `engines` as well, which is what hands it the items.

An item is one JS function, with no trailing `;`. What it is called with, and what it returns, is decided by its engine. This one is called with `{ now }` and returns a `filter`:

```javascript
({ now }) => ({
  filter: { status: 'open', created_at: { $lte: now } },
})
```

An engine is an ordinary script. It reads its items from the `global` variable: an array of `{ name, build }` in plan order, where `build` is the item's function:

```javascript
(() => {
  const options = globalThis.MONGO_ANALYSIS_OPTIONS || {};
  const orders = db.getCollection(options.collection);
  const results = globalThis.ORDER_QUERIES.map(({ name, build }) => {
    const query = build({ now: new Date() });
    const stats = orders
      .find(query.filter)
      .maxTimeMS(options.queryTimeoutMS)
      .limit(options.explainLimit)
      .explain('executionStats').executionStats;
    return { name, nReturned: stats.nReturned, docsExamined: stats.totalDocsExamined };
  });
  print(`ANALYZER_RESULT_JSON:${EJSON.stringify(results)}`);
})();
```

An item shows up in `make db-analyzer-list` as `collections/<collection>/<group>/<item>` and works even when `items` does not list it. A folder that no `plan.json` declares as a group is not treated as scripts.
