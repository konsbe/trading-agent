#!/usr/bin/env bash
# Golden check for momentum-scanner and momentum-tracker.
#
# Runs scanner then tracker, as momentum-daily does, against a scratch
# database (trading_golden, dropped and recreated from the migrations for each
# run) seeded with testdata/momentum_golden/fixture.sql, and compares what they
# wrote (testdata/momentum_golden/dump.sql) and printed:
#
#   1. BASE_REF's binaries, no flags                 -> must equal golden.txt
#   2. this tree's binaries, no flags                -> must equal golden.txt,
#                                                       and print what BASE printed
#   3. this tree's binaries, -session D, with data
#      that landed after D already stored            -> must equal golden.txt,
#                                                       and print what BASE printed
#
# So the unattended path (no flag) is proven unchanged against the committed
# code, and a caught-up session is proven to read nothing stamped after it.
# Printed lines are compared sorted: both commands print in map order.
#
#   scripts/momentum_golden.sh                  # BASE_REF=HEAD
#   BASE_REF=<commit> scripts/momentum_golden.sh
#   UPDATE=1 scripts/momentum_golden.sh         # rewrite golden.txt from BASE_REF
#
# Needs the ta-phase1 container (GOLDEN_PG_CONTAINER) and Go.
set -euo pipefail

cd "$(dirname "$0")/.."
root=$(git rev-parse --show-toplevel)
base_ref=${BASE_REF:-HEAD}
pg=${GOLDEN_PG_CONTAINER:-ta-phase1}
db=trading_golden
dsn=${GOLDEN_DATABASE_URL:-postgres://postgres:pg@localhost:55442/$db}
fixture=testdata/momentum_golden
golden=$fixture/golden.txt
d=$(date -u +%F)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

psql_db() { docker exec -i "$pg" psql -U postgres -d "$db" -q -v ON_ERROR_STOP=1 "$@"; }

reset_db() { # $1 = later (0|1)
	docker exec "$pg" psql -U postgres -q -c "DROP DATABASE IF EXISTS $db" -c "CREATE DATABASE $db" >/dev/null
	for f in "$root"/shared/databases/migrations/[0-9]*.sql; do
		# 013's cohort insert needs live universe rows, which an empty database
		# lacks; its tables are still created, and neither command reads them.
		if [[ $(basename "$f") == 013_* ]]; then
			docker exec -i "$pg" psql -U postgres -d "$db" -q >/dev/null 2>&1 <"$f" || true
			continue
		fi
		psql_db >/dev/null <"$f"
	done
	psql_db -v D="$d" -v later="$1" >/dev/null <"$fixture/fixture.sql"
}

run_chain() { # $1 = bin dir, $2 = out prefix, rest = flags
	local bin=$1 out=$2
	shift 2
	DATABASE_URL=$dsn "$bin/momentum-scanner" "$@" >"$out.stdout"
	DATABASE_URL=$dsn "$bin/momentum-tracker" "$@" >>"$out.stdout"
	psql_db -v D="$d" <"$fixture/dump.sql" >"$out.dump"
	grep -v '^point-in-time session:' "$out.stdout" | sort >"$out.sorted"
}

echo "building $base_ref and the working tree"
mkdir -p "$work/base-src" "$work/base" "$work/new"
git -C "$root" archive "$base_ref" services/data-analyzer | tar -x -C "$work/base-src"
(cd "$work/base-src/services/data-analyzer" &&
	go build -o "$work/base/" ./cmd/momentum-scanner ./cmd/momentum-tracker)
go build -o "$work/new/" ./cmd/momentum-scanner ./cmd/momentum-tracker

echo "session D = $d"
reset_db 0
run_chain "$work/base" "$work/base"
if [[ ${UPDATE:-} == 1 ]]; then
	cp "$work/base.dump" "$golden"
	rows=$(wc -l <"$golden")
	echo "wrote $golden from $base_ref: $rows rows"
fi

fail=0
check() { # $1 = label, $2 = got, $3 = want
	if diff -u "$3" "$2" >"$work/diff"; then
		echo "  ok    $1"
	else
		echo "  FAIL  $1"
		head -40 "$work/diff"
		fail=1
	fi
}

check "base ($base_ref), no flag: writes = golden" "$work/base.dump" "$golden"

reset_db 0
run_chain "$work/new" "$work/new"
check "working tree, no flag: writes = golden" "$work/new.dump" "$golden"
check "working tree, no flag: output = base" "$work/new.sorted" "$work/base.sorted"

reset_db 1
run_chain "$work/new" "$work/pit" -session "$d"
check "working tree, -session D after D+1 landed: writes = golden" "$work/pit.dump" "$golden"
check "working tree, -session D after D+1 landed: output = base" "$work/pit.sorted" "$work/base.sorted"

docker exec "$pg" psql -U postgres -q -c "DROP DATABASE IF EXISTS $db" >/dev/null
exit $fail
