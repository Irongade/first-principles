#!/bin/bash
# Create garbage (overwrites + deletes), compact, check the garbage is gone
# but the live data survived.

echo "== writing 50 keys, 5 times each (49 of every 50 records is dead) =="
for round in {1..5}; do
  for i in {1..50}; do
    value=$(head -c 500 /dev/urandom | base64 | tr -d '\n')
    curl -s -o /dev/null -X PUT http://localhost:8080/kv/key$i \
      -H "Content-Type: application/json" \
      -d "{\"value\":\"round$round-$value\"}"
  done
  echo "  round $round done"
done

echo
echo "== deleting keys 1-10 =="
for i in {1..10}; do
  curl -s -o /dev/null -X DELETE http://localhost:8080/kv/key$i
done

echo
echo "before compaction:"
echo "  segments: $(ls -1 ./data/segment-*.log 2>/dev/null | wc -l)"
echo "  size:     $(du -sh ./data | cut -f1)"

echo
echo "== POST /compaction =="
curl -s -w "\n  status %{http_code}\n" -X POST http://localhost:8080/compaction

echo
echo "after compaction:"
echo "  segments: $(ls -1 ./data/segment-*.log 2>/dev/null | wc -l)"
echo "  size:     $(du -sh ./data | cut -f1)"
ls -lh ./data/

echo
echo "== live keys 11-50 should all be round5 =="
for i in {11..50}; do
  got=$(curl -s http://localhost:8080/kv/key$i)
  if echo "$got" | grep -q "round5"; then
    echo "  ok   key$i"
  else
    echo "  BAD  key$i -> ${got:0:60}"
  fi
done

echo
echo "== deleted keys 1-10 should be 404 =="
for i in {1..10}; do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/kv/key$i)
  if [ "$code" = "404" ]; then
    echo "  ok   key$i -> 404"
  else
    echo "  BAD  key$i -> $code (tombstone lost or resurrected)"
  fi
done