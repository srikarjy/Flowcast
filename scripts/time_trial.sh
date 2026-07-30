#!/usr/bin/env bash
# Stopwatch for docs/user_testing_protocol.md. Starts timing on launch,
# stops on Enter, appends one row to user_testing_results.csv.
#
# Usage: scripts/time_trial.sh <subject_id> <condition: manual|flowcast>
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <subject_id> <condition: manual|flowcast>" >&2
  exit 2
fi

subject="$1"
condition="$2"
out="user_testing_results.csv"

if [ ! -f "$out" ]; then
  echo "subject,condition,seconds,timestamp" > "$out"
fi

start=$(date +%s)
echo "Timing started for subject=$subject condition=$condition. Press Enter when the subject answers."
read -r _

end=$(date +%s)
elapsed=$((end - start))
timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "${subject},${condition},${elapsed},${timestamp}" >> "$out"
echo "Recorded: ${elapsed}s -> $out"
