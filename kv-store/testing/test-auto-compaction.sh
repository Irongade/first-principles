#!/bin/bash
# Automatic (ticker) compaction. Start the server like this first:
#   COMPACTION_INTERVAL=5s go run .
#
# Then: COMPACTION_INTERVAL=5s ./test-compaction-auto.sh

INTERVAL=${COMPACTION_INTERVAL:-5s}
SECS=$(echo "$INTERVAL" | sed 's/s$//')
WAIT=$(( SECS * 4 + 10 ))       # a few intervals of slack

echo "interval=$INTERVAL, will poll for up to ${WAIT}s"

echo
echo "== writing 50 keys, 5 rounds each =="
for round in {1..5}; do
  for i in {1..50}; do
    value=$(head -c 500 /dev/urandom | base64 | tr -d '\n')
    curl -s -o /dev/null -X PUT http://localhost:8080/kv/key$i \
      -H "Content-Type: application/json" \
      -d "{\"value\":\"round$round-$value\"}"
  done
done

for i in {1..10}; do
  curl -s -o /dev/null -X DELETE http://localhost:8080/kv/key$i
done

before_segments=$(ls -1 ./data/segment-*.log 2>/dev/null | wc -l)
before_bytes=$(cat ./data/segment-*.log | wc -c)
echo "before: $before_segments segments, $before_bytes bytes"

echo
echo "== polling, no writes from here on =="
compacted=no
for t in $(seq 1 $WAIT); do
  sleep 1
  now_segments=$(ls -1 ./data/segment-*.log 2>/dev/null | wc -l)
  now_bytes=$(cat ./data/segment-*.log 2>/dev/null | wc -c)
  if [ "$now_segments" -lt "$before_segments" ]; then
    echo "  fired at ${t}s: $now_segments segments, $now_bytes bytes"
    compacted=yes
    break
  fi
  [ $((t % 5)) -eq 0 ] && echo "  ${t}s... $now_segments segments"
done

if [ "$compacted" = "no" ]; then
  echo "  NOTHING HAPPENED in ${WAIT}s"
  echo "  -> is startCompactionLoop actually called? is compactor non-nil"
  echo "     (needs PositionIndex)? did Add(1) land before the go statement?"
fi

echo
echo "== waiting one more interval to check it repeats =="
sleep $(( SECS + 2 ))
again_segments=$(ls -1 ./data/segment-*.log 2>/dev/null | wc -l)
echo "  segments now: $again_segments (was $now_segments)"
echo "  -> ticker should keep running; a one-shot timer would be a bug"

echo
echo "== live keys 11-50 should be round5 =="
bad=0
for i in {11..50}; do
  got=$(curl -s http://localhost:8080/kv/key$i)
  echo "$got" | grep -q "round5" || { echo "  BAD  key$i -> ${got:0:60}"; bad=1; }
done
[ $bad -eq 0 ] && echo "  all ok"

echo
echo "== deleted keys 1-10 should be 404 =="
bad=0
for i in {1..10}; do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/kv/key$i)
  [ "$code" = "404" ] || { echo "  BAD  key$i -> $code"; bad=1; }
done
[ $bad -eq 0 ] && echo "  all ok"