#!/usr/bin/env bash
# minimal "runner" for macos/linux when c++ binary is not built.
# worker calls: runner.sh --job-id x --type y --payload z
# we echo ok:<payload> and exit 0 so the job completes successfully.
# on mac/linux: chmod +x scripts/runner.sh then set execution_binary to its path.


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





