#!/usr/bin/env bash
# curl commands for every endpoint, including error cases.
# Run in Git Bash from the project root, with the API running:
#
#   bash scripts/curl-examples.sh
#   BASE=http://localhost:8081 bash scripts/curl-examples.sh
#
# Or copy any single command into Git Bash. Each prints the body,
# then the HTTP status code in [brackets].

BASE="${BASE:-http://localhost:8080}"
DATE="${DATE:-2026-11-03}"
JSON='Content-Type: application/json'
S='\n  [%{http_code}]\n\n'
NAME="Test-$(date +%s)" # unique machine name for this run

echo "== Health"
curl -s -w "$S" "$BASE/health"

echo "== POST /machines: create (201)"
curl -s -w "$S" -X POST "$BASE/machines" -H "$JSON" \
  -d "{\"name\":\"$NAME\",\"line\":\"Line A\"}"

echo "== POST /machines: same name again (409)"
curl -s -w "$S" -X POST "$BASE/machines" -H "$JSON" \
  -d "{\"name\":\"$NAME\",\"line\":\"Line A\"}"

echo "== POST /machines: missing line (400)"
curl -s -w "$S" -X POST "$BASE/machines" -H "$JSON" -d '{"name":"CNC-99"}'

echo "== POST /machines: blank name (400)"
curl -s -w "$S" -X POST "$BASE/machines" -H "$JSON" -d '{"name":"   ","line":"Line A"}'

echo "== POST /machines: invalid JSON (400)"
curl -s -w "$S" -X POST "$BASE/machines" -H "$JSON" -d '{ this is not json'

echo "== GET /machines (200)"
curl -s -w "$S" "$BASE/machines"

echo "== POST /downtime: create, 08:00:00 to 08:45:30 -> minutes 45 (201)"
curl -s -w "$S" -X POST "$BASE/downtime" -H "$JSON" \
  -d '{"machine_id":1,"reason":"No material","start_time":"2026-11-04T08:00:00+05:30","end_time":"2026-11-04T08:45:30+05:30"}'

echo "== POST /downtime: unknown machine_id (404)"
curl -s -w "$S" -X POST "$BASE/downtime" -H "$JSON" \
  -d '{"machine_id":999999,"reason":"Breakdown","start_time":"2026-11-04T08:00:00Z","end_time":"2026-11-04T08:45:00Z"}'

echo "== POST /downtime: end_time before start_time (400)"
curl -s -w "$S" -X POST "$BASE/downtime" -H "$JSON" \
  -d '{"machine_id":1,"reason":"Breakdown","start_time":"2026-11-04T09:00:00Z","end_time":"2026-11-04T08:45:00Z"}'

echo "== POST /downtime: invalid reason (400)"
curl -s -w "$S" -X POST "$BASE/downtime" -H "$JSON" \
  -d '{"machine_id":1,"reason":"Coffee break","start_time":"2026-11-04T08:00:00Z","end_time":"2026-11-04T08:45:00Z"}'

echo "== POST /downtime: bad time format (400)"
curl -s -w "$S" -X POST "$BASE/downtime" -H "$JSON" \
  -d '{"machine_id":1,"reason":"Breakdown","start_time":"04/11/2026 08:00","end_time":"2026-11-04T08:45:00Z"}'

echo "== POST /downtime: missing fields (400)"
curl -s -w "$S" -X POST "$BASE/downtime" -H "$JSON" -d '{"reason":"Breakdown"}'

echo "== GET /downtime?machine_id=1&date=$DATE (200)"
curl -s -w "$S" "$BASE/downtime?machine_id=1&date=$DATE"

echo "== GET /downtime?date=2026-13-40: bad date (400)"
curl -s -w "$S" "$BASE/downtime?date=2026-13-40"

echo "== GET /downtime?machine_id=abc: bad machine_id (400)"
curl -s -w "$S" "$BASE/downtime?machine_id=abc"

echo "== GET /reports/daily?date=$DATE: concurrent (200)"
curl -s -w "$S" "$BASE/reports/daily?date=$DATE"

echo "== GET /reports/daily?date=$DATE&mode=sequential (200)"
curl -s -w "$S" "$BASE/reports/daily?date=$DATE&mode=sequential"

echo "== GET /reports/daily?date=2020-01-01: no data (200, empty report)"
curl -s -w "$S" "$BASE/reports/daily?date=2020-01-01"

echo "== GET /reports/daily: missing date (400)"
curl -s -w "$S" "$BASE/reports/daily"

echo "== GET /reports/daily?date=2026-02-30: bad date (400)"
curl -s -w "$S" "$BASE/reports/daily?date=2026-02-30"

echo "== GET /reports/daily?mode=fast: bad mode (400)"
curl -s -w "$S" "$BASE/reports/daily?date=$DATE&mode=fast"

echo "== GET /does-not-exist: unknown route (404)"
curl -s -w "$S" "$BASE/does-not-exist"
