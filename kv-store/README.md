# kvstore

A key-value store built from scratch in Go, as a hands-on way to learn how
storage engines actually work under the hood — not by reading about
Bitcask/LSM-trees, but by building a small, real one and hitting every
sharp edge along the way.

## What this is

An HTTP-accessible KV store backed by an append-only log on disk, with:

- **Segmented, rotating storage** — the log is split into bounded-size
  files instead of one ever-growing file.
- **A pluggable in-memory index** — either the full value lives in memory
  (fast, RAM-bound), or just a pointer to its location on disk
  (slower per read, disk-bound capacity).
- **Background compaction** — a periodic pass that reclaims space from
  overwritten and deleted keys, without freezing live traffic while it runs.
- **Crash-safe recovery** — the index is never persisted; it's rebuilt by
  replaying the log on startup, which means the log itself is the only
  source of truth.
- **Environment-driven configuration** — every tunable (segment size,
  sync policy, compaction cadence, storage/index strategy) is overridable
  via env vars, with sane defaults when unset.

## Why it's built this way

This design is a small version of what Bitcask, and the log-structured
core of engines like RocksDB/LevelDB, actually do:

| Concept here                                 | Real-world analog                                            |
| -------------------------------------------- | ------------------------------------------------------------ |
| Append-only segment files                    | Write-ahead logs (Postgres WAL, Kafka's log segments)        |
| In-memory index of key → disk location       | Bitcask's keydir                                             |
| Segment rotation at a size threshold         | Kafka's log segment rolling                                  |
| Compaction merging/deduping stale segments   | LSM-tree compaction, tombstone GC                            |
| Rebuilding the index from the log on startup | Standard WAL replay / crash recovery                         |
| Background compaction on a timer             | Postgres autovacuum, RocksDB's background compaction threads |

None of this is distributed — there's one node, one writer, no
replication — but the _single-node_ storage problems it solves (durability
vs. throughput, space reclamation, crash recovery, avoiding maintenance
work on the hot path) are the same ones every real storage engine has to
solve before distribution even enters the picture.

## Architecture

```


    HTTP request
         │
         ▼
    api.Engine  ──── routes GET/PUT/DELETE on /kv/:key,
         │            POST /compaction, GET /health
         ▼
  storage.FileStore ──── owns the lock, owns the index,
    │      │    │        orchestrates everything below
    │      │    └──────────────────┐
    ▼      ▼                       ▼

FileWriter FileReader          compaction.Compactor
(append to  (read by            (merges stale segments,
active      offset,             runs on its own timer,
segment)    rebuild index       commits results back through FileStore)
            on startup)

```

FileWriter FileReader compaction.Compactor
(append to (read by (merges stale segments,
active offset, runs on its own timer,
segment) rebuild index commits results back
on startup) through FileStore)

`FileStore` is the only thing that holds the lock guarding shared state.
`Compactor` never touches the index or the lock directly — it hands its
result back to `FileStore`, which applies it under lock. That boundary
exists on purpose: it's what keeps a background maintenance process from
being able to race with a live write in a way nobody can reason about.

## Storage format

Each record is a single line: `version|operation|key|value`, with `|`,
`\`, and newlines escaped inside fields. Deletes are tombstone records —
a `del` operation with no value — rather than actually removing bytes
from the log at write time. Segments are named `segment-<zero-padded-id>.log`,
and ascending ID order is a load-bearing invariant throughout: it's what
lets a full rebuild figure out, for any key with multiple writes scattered
across segments, which one is actually the latest.

## Compaction

Compaction reads the segments that are neither the currently-active one
nor the one immediately before it, keeps only the latest surviving record
per key (dropping tombstones entirely, since compacting the whole stale
prefix means there's nothing older left for a tombstone to keep shadowing),
and writes the survivors into new segment(s) that **reuse the highest IDs
among the segments they replace** — never freshly-minted higher IDs. That
detail matters: if compacted output ever got a _higher_ ID than the
segment it excluded, a later index rebuild would see it as "newer" than
data that's actually more recent, and silently serve stale values. Reusing
old IDs keeps "higher ID = more recent" true forever, regardless of how
much compaction happens over the store's lifetime.

The expensive part (scanning, deduping, writing merged output) runs
without holding the store's lock. Only the final commit — renaming the
merged files into place and updating the live index — runs under lock,
and even then only after checking each key hasn't been overwritten by a
newer write that raced ahead during the scan.

## Configuration

See `.env.example` for the full list of environment variables — segment
size, buffer size, sync policy, compaction threshold/interval, and which
storage/index strategy to use. Every variable is optional; unset values
fall back to built-in defaults, and unparseable values log a warning and
fall back rather than crashing the process.

## Running it

go run main.go

Copy `.env.example` to `.env` and adjust as needed — or use the smaller,
faster values from the testing `.env` if you want segment rotation and
compaction to be observable within seconds rather than needing megabytes
of data.

## API

| Method   | Path          | Does                                     |
| -------- | ------------- | ---------------------------------------- |
| `GET`    | `/kv/{key}`   | fetch a value                            |
| `PUT`    | `/kv/{key}`   | store a value (body: `{"value": "..."}`) |
| `DELETE` | `/kv/{key}`   | tombstone a key                          |
| `POST`   | `/compaction` | trigger a compaction pass on demand      |
| `GET`    | `/health`     | liveness check                           |
