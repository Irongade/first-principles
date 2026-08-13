# Key learning points

Things that turned out to matter more than expected while building this,
roughly in the order they showed up.

## 1. Buffering and offsets are the same problem twice

The first real bug wasn't in the storage design — it was that the file
writer's offset only advanced _after_ an early return for the sync-every-write
case, so every record reported the same offset. The lesson wasn't really
about buffering; it was that **state mutation has to happen on every code
path, not just the one you were looking at when you wrote it**. The same
shape of bug (a value computed correctly, then thrown away before it
reached anywhere that mattered) showed up again much later, in a much
subtler form — see #5.

## 2. `RWMutex` doesn't mean what it sounds like until you use both halves

Declaring a lock as `sync.RWMutex` doesn't buy you concurrent reads —
`Get` calling `.Lock()` instead of `.RLock()` made it fully serialized
against everything else the whole time, silently. And later, pairing
`RLock()` with `Unlock()` (instead of `RUnlock()`) is a runtime `fatal`
error, not a panic — unrecoverable, kills the process outright on the
very first call. Concurrency primitives punish small mismatches much more
harshly than most other mistakes do.

## 3. An invariant is worth more than a clever check

The single idea that made compaction actually work was deciding, early,
that segment IDs must always increase with recency — and holding that
invariant sacred through every later change, including how compacted
output gets its ID. Once that was fixed, huge classes of "what if a write
races compaction" bugs resolved into a one-line comparison
(`existing segment ID > this segment ID → skip`) instead of needing
elaborate coordination. Good invariants make hard problems boring.

## 4. Crash safety is about _order_, not extra machinery

Making compaction crash-safe didn't need transactions or a write-ahead
log of its own — it needed the right _sequence_: install new data before
removing old data, and never delete anything the live index might still
be pointing at. Because the in-memory index is never persisted and always
gets rebuilt from disk on startup, a crash at almost any point just means
"redo a bit of work," never "lose data" — as long as delete always comes
after install, not before.

## 5. "Looks correct" and "is correct" are different in Go specifically

around map value semantics

The trickiest bug in the whole project: code that computed the exact
right offset, at the exact right time, and then assigned it to a `range`
loop's copy of a map entry instead of writing it back into the map. It
compiled, it read naturally, and it silently did nothing. This is a
uniquely Go trap (maps of structs, not pointers) but the general lesson
is broader — **when a computed value needs to end up somewhere specific,
verify it actually landed there, not just that the computation looks
right.**

## 6. Two-phase design beats one big lock

Splitting compaction into an unlocked "do the expensive work" phase and a
short locked "commit the result" phase wasn't just a performance
optimization — it changed what needed to be true for correctness. The
expensive phase could safely ignore concurrent writes entirely (since
it only reads immutable, already-sealed segments); only the commit phase
needed to worry about races, and only for as long as it actually takes to
swap a handful of map entries and rename a few files.

## 7. A background loop needs to _wait_, not just _signal_

Closing a channel to tell a goroutine to stop doesn't stop it — it stops
it the next time that goroutine checks. If shutdown proceeds immediately
after signaling, without waiting for confirmation the loop actually
exited, you can tear down resources a still-running compaction is mid-way
through using. The fix is cheap (a `WaitGroup`), but only if you remember
a signal and a guarantee are different things.

## 8. Defaults duplicated across files drift, quietly

Four different config-construction functions each had their own hardcoded
"default buffer size" — and two of them disagreed, for months, with
nothing ever surfacing it because nothing compared them. Centralizing
config loading wasn't just about adding environment variables; it was the
first time anything forced all those numbers to be looked at side by side.

## 9. Real bugs hide behind partially-correct fixes

Fixing the segment-ID resolution for compaction (a real, necessary fix)
made the symptom _look_ gone — some keys started working — while the
offset/size half of the same bug was still silently broken underneath it.
A fix that changes the symptom without being traced all the way through
to "why does this specific value end up correct" is only half-verified.
