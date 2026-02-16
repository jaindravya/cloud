#!/usr/bin/env bash
# minimal "runner" for macOS/Linux when C++ binary is not built.
# worker calls: runner.sh --job-id X --type Y --payload Z
# we echo OK:<payload> and exit 0 so the job completes successfully.
# on Mac/Linux: chmod +x scripts/runner.sh then set EXECUTION_BINARY to its path.


payload=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --job-id)  shift; shift ;;   # skip value
    --type)    shift; shift ;;
    --payload) shift; payload="$1"; shift ;;
    *)         shift ;;
  esac
done


echo "OK:$payload"
exit 0





