#!/bin/bash
# Write 100 keys with ~670-char values, then look at the segment files.

echo "segments before:"
ls -l ./data/segment-*.log 2>/dev/null | wc -l

for i in {1..100}; do
  value=$(head -c 500 /dev/urandom | base64 | tr -d '\n')
  curl -s -o /dev/null -X PUT http://localhost:8080/kv/key$i \
    -H "Content-Type: application/json" \
    -d "{\"value\":\"$value\"}"
done

echo "PUTs done"
echo
echo "segments after:"
ls -lh ./data/segment-*.log