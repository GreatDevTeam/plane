#!/usr/bin/env bash
# Elasticsearch query helper for jobscanner logs
# Indices: jobscanner_site-YYYY.WW (app logs), .ds-api-logs-* (API logs)
#
# Usage: docs/elastic.sh <command> [options]
# Env:   ELASTIC_URL (default: http://10.10.0.27:9200)
#
# Common filters (work on errors, tail, search):
#   --level ERROR        filter by level_name
#   --instance us        filter by instance_code
#   --code php           filter by code
#   --website jobscanner filter by website
#   --query 'level_name.keyword:"ERROR" AND message:"fail"'  full ES query_string

set -euo pipefail

ELASTIC_URL="${ELASTIC_URL:-http://10.10.0.27:9200}"
APP_INDEX="jobscanner_site-*"

_es_post() {
    curl -s -X POST "$ELASTIC_URL/$1/_search" \
        -H 'Content-Type: application/json' \
        -d "$2"
}

# Python hit printer — pass "reverse" or "normal" as first arg
# Reads ES response JSON from stdin
PRINT_HITS='
import json, sys
reverse = sys.argv[1] == "reverse"
data = json.load(sys.stdin)
hits = data.get("hits", {}).get("hits", [])
total = data.get("hits", {}).get("total", {}).get("value", 0)
print(f"Total: {total:,}")
if reverse:
    hits = list(reversed(hits))
for h in hits:
    s = h["_source"]
    ts   = s.get("@timestamp", "")[:19].replace("T", " ")
    lvl  = s.get("level_name", "-")
    inst = s.get("instance_code", "-")
    code = s.get("code", "-")
    site = s.get("website", "-")
    msg  = s.get("message", "")[:150]
    print(f"{ts}  [{lvl}] [{inst}] [{code}] [{site}]  {msg}")
'

SOURCE_FIELDS='["@timestamp", "level_name", "instance_code", "code", "website", "message"]'

_parse_common_flags() {
    LEVEL="${LEVEL:-}"
    INSTANCE="${INSTANCE:-}"
    CODE="${CODE:-}"
    WEBSITE="${WEBSITE:-}"
    QUERY_STR="${QUERY_STR:-}"
    EXTRA_ARGS=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --level)    LEVEL="$2";     shift 2 ;;
            --instance) INSTANCE="$2";  shift 2 ;;
            --code)     CODE="$2";      shift 2 ;;
            --website)  WEBSITE="$2";   shift 2 ;;
            --query)    QUERY_STR="$2"; shift 2 ;;
            *)          EXTRA_ARGS+=("$1"); shift ;;
        esac
    done
}

_build_filters() {
    local parts=()
    [[ -n "${LEVEL:-}"     ]] && parts+=("{\"term\": {\"level_name.keyword\": \"$LEVEL\"}}")
    [[ -n "${INSTANCE:-}"  ]] && parts+=("{\"term\": {\"instance_code.keyword\": \"$INSTANCE\"}}")
    [[ -n "${CODE:-}"      ]] && parts+=("{\"term\": {\"code.keyword\": \"$CODE\"}}")
    [[ -n "${WEBSITE:-}"   ]] && parts+=("{\"term\": {\"website.keyword\": \"$WEBSITE\"}}")
    if [[ -n "${QUERY_STR:-}" ]]; then
        local escaped="${QUERY_STR//\"/\\\"}"
        parts+=("{\"query_string\": {\"query\": \"$escaped\", \"default_field\": \"message\"}}")
    fi
    local joined
    joined=$(printf '%s,' "${parts[@]+"${parts[@]}"}")
    echo "[${joined%,}]"
}

cmd="${1:-help}"
shift || true

case "$cmd" in
  errors)
    LEVEL="ERROR"
    _parse_common_flags "$@"
    set -- "${EXTRA_ARGS[@]+"${EXTRA_ARGS[@]}"}"
    N="${1:-20}"
    FILTERS=$(_build_filters)
    BODY="{\"size\": $N, \"sort\": [{\"@timestamp\": \"desc\"}], \"query\": {\"bool\": {\"filter\": $FILTERS}}, \"_source\": $SOURCE_FIELDS}"
    _es_post "$APP_INDEX" "$BODY" | python3 -c "$PRINT_HITS" "normal"
    ;;

  search)
    _parse_common_flags "$@"
    set -- "${EXTRA_ARGS[@]+"${EXTRA_ARGS[@]}"}"
    if [[ $# -gt 0 && -z "${QUERY_STR:-}" ]]; then
        QUERY_STR="$1"; shift
    fi
    [[ -z "${QUERY_STR:-}" ]] && { echo "Usage: elastic.sh search <query> [N] [filters...]"; exit 1; }
    N="${1:-20}"
    FILTERS=$(_build_filters)
    BODY="{\"size\": $N, \"sort\": [{\"@timestamp\": \"desc\"}], \"query\": {\"bool\": {\"filter\": $FILTERS}}, \"_source\": $SOURCE_FIELDS}"
    _es_post "$APP_INDEX" "$BODY" | python3 -c "$PRINT_HITS" "normal"
    ;;

  tail)
    _parse_common_flags "$@"
    set -- "${EXTRA_ARGS[@]+"${EXTRA_ARGS[@]}"}"
    N="${1:-20}"
    FILTERS=$(_build_filters)
    BODY="{\"size\": $N, \"sort\": [{\"@timestamp\": \"desc\"}], \"query\": {\"bool\": {\"filter\": $FILTERS}}, \"_source\": $SOURCE_FIELDS}"
    _es_post "$APP_INDEX" "$BODY" | python3 -c "$PRINT_HITS" "reverse"
    ;;

  stats)
    PERIOD="${1:-24h}"
    BODY="{\"size\": 0, \"query\": {\"range\": {\"@timestamp\": {\"gte\": \"now-$PERIOD\"}}}, \"aggs\": {\"by_level\": {\"terms\": {\"field\": \"level_name.keyword\", \"size\": 10}}, \"by_instance\": {\"terms\": {\"field\": \"instance_code.keyword\", \"size\": 20}}, \"by_code\": {\"terms\": {\"field\": \"code.keyword\", \"size\": 10}}, \"by_website\": {\"terms\": {\"field\": \"website.keyword\", \"size\": 10}}}}"
    _es_post "$APP_INDEX" "$BODY" | python3 -c '
import json, sys
data = json.load(sys.stdin)
aggs = data.get("aggregations", {})
def print_agg(label, key):
    buckets = aggs.get(key, {}).get("buckets", [])
    if not buckets:
        return
    print(f"{label}:")
    for b in sorted(buckets, key=lambda x: -x["doc_count"]):
        print(f"  {b[\"key\"]:<16} {b[\"doc_count\"]:>10,}")
    print()
print_agg("By level",    "by_level")
print_agg("By instance", "by_instance")
print_agg("By code",     "by_code")
print_agg("By website",  "by_website")
'
    ;;

  indices)
    curl -s "$ELASTIC_URL/_cat/indices/jobscanner_site-*?h=index,docs.count,store.size&s=index"
    ;;

  *)
    echo "Usage: docs/elastic.sh <command> [options]"
    echo ""
    echo "Commands:"
    echo "  errors  [N] [filters]          last N errors (default: 20)"
    echo "  search  <query> [N] [filters]  ES query_string search"
    echo "  tail    [N] [filters]          recent logs oldest-first (default: 20)"
    echo "  stats   [period]               breakdown by level/instance/code/website (default: 24h)"
    echo "  indices                        list app log indices"
    echo ""
    echo "Filters (work on errors, search, tail):"
    echo "  --level ERROR"
    echo "  --instance us"
    echo "  --code php"
    echo "  --website jobscanner"
    echo '  --query '\''level_name.keyword:"ERROR" AND NOT message:"Jobs found"'\'''
    echo ""
    echo "Examples:"
    echo "  docs/elastic.sh errors 50 --instance us"
    echo "  docs/elastic.sh search 'RoadRunner queue dispatch failed' 10"
    echo '  docs/elastic.sh search --query '\''level_name.keyword:"ERROR" AND NOT message:"Jobs found"'\'' 20'
    echo "  docs/elastic.sh tail 30 --level ERROR --instance uk --code php"
    echo "  docs/elastic.sh stats 1h"
    ;;
esac
