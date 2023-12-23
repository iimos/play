#!/bin/bash

# trap "exit 1" INT

# function watch() {
#     inotifywait -q -m -e close_write . | while read events; do kill $(pidof strace_view) || true; done
# }

# while true; do go run /proj/src/github.com/iimos/play/strace_view/ rg 123; done

sigint_handler() {
  kill $PID
  exit
}

trap sigint_handler INT


while true; do
  echo "start"
  # $@ &
  go run /proj/src/github.com/iimos/play/strace_view/ rg 123 &
  PID=$!
  echo "PID=$PID"
  inotifywait -e modify -e move -e create -e delete -e attrib -r `pwd`
  kill $(pidof strace_view)
  kill $PID
  sleep 1
done
