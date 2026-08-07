#!/bin/bash
# Write 20 keys, then read all 20 back and check the values.

for i in {1..20}; do
  curl -s -o /dev/null -X PUT http://localhost:8080/kv/read$i \
    -H "Content-Type: application/json" \
    -d "{\"value\":\"value-$i\"}"
done

echo "wrote 20 keys, reading them back:"

for i in {1..20}; do
  got=$(curl -s http://localhost:8080/kv/read$i)
  if echo "$got" | grep -q "value-$i"; then
    echo "  ok   read$i"
  else
    echo "  BAD  read$i -> $got"
  fi
done

echo
echo "missing key should be 404:"
curl -s -o /dev/null -w "  got %{http_code}\n" http://localhost:8080/kv/nope