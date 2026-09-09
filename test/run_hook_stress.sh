#!/bin/bash
# Hook-system stability scenarios (h* plugin family). Each scenario is a
# standalone plugin that builds its own private pools and asserts PASS/FAIL
# invariants; any FAIL makes the scenario exit 1 and the script aborts.
# Run with the race detector:  run_hook_stress.sh race
#   (race builds a -race binary; slower, use it in CI to catch hook races)
# Per-plugin knobs: see `./agilepool_test --help hcount` etc.
set -e
if [ "$1" = "race" ]; then
    go build -race -o agilepool_test .
else
    go build -o agilepool_test .
fi

./agilepool_test --hcount
./agilepool_test --hpanic num=3000 stage=all
./agilepool_test --hpanic num=200 level=callback stage=all 2>/dev/null
./agilepool_test --horder
./agilepool_test --hctx
./agilepool_test --hblock num=2000 delay-us=100
./agilepool_test --hchurn
./agilepool_test --hreenter
./agilepool_test --hclose

echo "hook-stability suite: all PASS"
