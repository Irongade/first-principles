#!/bin/bash
# Write data, wait for you to restart the server, then check it's all still there.

# 10 normal keys
for i in {1..10}; do
  curl -s -o /dev/null -X PUT http://localhost:8080/kv/persist$i \
    -H "Content-Type: application/json" -d "{\"value\":\"p$i\"}"
done

# overwrite test: write A, fill up a segment, write B
curl -s -o /dev/null -X PUT http://localhost:8080/kv/mykey \
  -H "Content-Type: application/json" -d '{"value":"valueA"}'

for i in {1..10}; do
  value=$(head -c 500 /dev/urandom | base64 | tr -d '\n')
  curl -s -o /dev/null -X PUT http://localhost:8080/kv/filler-a-$i \
    -H "Content-Type: application/json" -d "{\"value\":\"$value\"}"
done

curl -s -o /dev/null -X PUT http://localhost:8080/kv/mykey \
  -H "Content-Type: application/json" -d '{"value":"valueB"}'

# delete test: write it, fill up a segment, delete it
curl -s -o /dev/null -X PUT http://localhost:8080/kv/doomed \
  -H "Content-Type: application/json" -d '{"value":"gone"}'

for i in {1..10}; do
  value=$(head -c 500 /dev/urandom | base64 | tr -d '\n')
  curl -s -o /dev/null -X PUT http://localhost:8080/kv/filler-b-$i \
    -H "Content-Type: application/json" -d "{\"value\":\"$value\"}"
done

curl -s -o /dev/null -X DELETE http://localhost:8080/kv/doomed

echo "segments written:"
ls -1 ./data/segment-*.log

echo
read -p ">> Restart the server (Ctrl+C, then start again), then press Enter. "

echo
echo "checking keys:"
for i in {1..10}; do
  got=$(curl -s http://localhost:8080/kv/persist$i)
  if echo "$got" | grep -q "p$i"; then
    echo "  ok   persist$i"
  else
    echo "  BAD  persist$i -> $got"
  fi
done

echo
echo "mykey should be valueB (latest write wins):"
curl -s http://localhost:8080/kv/mykey
echo

echo "doomed should be 404 (tombstone survived):"
curl -s -o /dev/null -w "  got %{http_code}\n" http://localhost:8080/kv/doomed