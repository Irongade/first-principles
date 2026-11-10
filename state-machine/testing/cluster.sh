#!/usr/bin/env bash
# Spin up N independent state-machine nodes locally.
# Usage: testing/cluster.sh start [N] | stop | status | clean
#
# Overridable via environment:
#   MODULE_DIR  path to the state-machine module   (default: parent of this script)
#   DATA_ROOT   where node-N data dirs live        (default: first_principles/data)
#   RUN_DIR     binary, pid files and logs         (default: testing/run)
#   BASE_PORT   node-1 port, node-i gets BASE+i-1  (default: 8081)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="${MODULE_DIR:-$SCRIPT_DIR/..}"
DATA_ROOT="${DATA_ROOT:-$MODULE_DIR/../data}"
RUN_DIR="${RUN_DIR:-$SCRIPT_DIR/run}"
BASE_PORT="${BASE_PORT:-8081}"
BIN="$RUN_DIR/node"

mkdir -p "$RUN_DIR"

port_for() { echo $((BASE_PORT + $1 - 1)); }
pidfile()  { echo "$RUN_DIR/node-$1.pid"; }

running_count() {
  local c=0 f
  for f in "$RUN_DIR"/node-*.pid; do
    [ -e "$f" ] && c=$((c + 1))
  done
  echo "$c"
}

start() {
  local n="$1" i port peers=""
  if [ "$(running_count)" != "0" ]; then
    echo "nodes already running; run 'stop' first" >&2
    exit 1
  fi

  (cd "$MODULE_DIR" && go build -o "$BIN" .)

  for i in $(seq 1 "$n"); do
    peers="${peers:+$peers,}$i=http://localhost:$(port_for "$i")"
  done

  for i in $(seq 1 "$n"); do
    port="$(port_for "$i")"
    NODE_ID="$i" NODE_LEADER="${LEADER_ID:-1}" NODE_PEERS="$peers" \
    NODE_DATA_DIR="$DATA_ROOT/node-$i" NODE_ADDR=":$port" \
      nohup "$BIN" > "$RUN_DIR/node-$i.log" 2>&1 &
    echo $! > "$(pidfile "$i")"
    echo "node-$i  starting on :$port  (pid $!)"
  done

  for i in $(seq 1 "$n"); do
    port="$(port_for "$i")"
    if ! curl -sf --retry 20 --retry-connrefused --retry-delay 1 \
        "localhost:$port/health" > /dev/null; then
      echo "node-$i failed to start, see $RUN_DIR/node-$i.log" >&2
      exit 1
    fi
  done
  echo "all $n nodes healthy, leader is node-${LEADER_ID:-1}"
}


stop() {
  local f pid
  for f in "$RUN_DIR"/node-*.pid; do
    [ -e "$f" ] || continue
    pid="$(cat "$f")"
    if kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid"
      while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
    fi
    rm -f "$f"
    echo "stopped $(basename "$f" .pid)"
  done
}

status() {
  local f name i pid port
  if [ "$(running_count)" = "0" ]; then
    echo "no nodes running"
    return
  fi
  for f in "$RUN_DIR"/node-*.pid; do
    name="$(basename "$f" .pid)"
    i="${name#node-}"
    pid="$(cat "$f")"
    port="$(port_for "$i")"
    if kill -0 "$pid" 2>/dev/null && curl -sf "localhost:$port/health" > /dev/null; then
      echo "$name  :$port  pid $pid  healthy"
    else
      echo "$name  :$port  pid $pid  DOWN"
    fi
  done
}

clean() {
  local d
  stop
  for d in "$DATA_ROOT"/node-*; do
    [ -d "$d" ] && rm -rf "$d" && echo "removed $d"
  done
  rm -rf "$RUN_DIR"
}

case "${1:-}" in
  start)  start "${2:-3}" ;;
  stop)   stop ;;
  status) status ;;
  clean)  clean ;;
  *)
    echo "usage: $0 start [N] | stop | status | clean" >&2
    exit 1
    ;;
esac
