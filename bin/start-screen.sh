#!/bin/bash
# Start one or more chromium-browser processes for testing Pijector.
# Recognizes the following env vars:
# - PIJECTOR_SCREEN_CHROMIUM : the path to the chromium-browser command to use.
#   This can also be the path to google-chrome.
# - PIJECTOR_SCREEN_DEFAULT_URL : the URL to which all kiosks should navigate
#   initially.
# - PIJECTOR_SCREEN_START_PORT : first port at which chromium debugger should
#   listen. Defaults to 9223, to avoid colliding with the Chrome Devtools default
#   port 9222.
# Takes the following arguments:
# - ARG1 (optional): the number of additional chromium screens to start. Defaults
#   to 0.
#
# Usage: start-screen.sh $NUM_SCREENS

: "${PIJECTOR_SCREEN_DEFAULT_URL:=http://localhost:9292/}"
: "${PIJECTOR_SCREEN_START_PORT:=9223}"
: "${PIJECTOR_SCREEN_CHROMIUM:=$(which chromium-browser)}"

if [[ "${PIJECTOR_SCREEN_CHROMIUM}" == "" ]] ; then
    echo "chromium-browser not found, cannot continue" 1>&2
    exit 1
fi

let _END_PORT="${PIJECTOR_SCREEN_START_PORT}+${1:-0}"
for _PORT in $(seq "${PIJECTOR_SCREEN_START_PORT}" "${_END_PORT}"); do
    "${PIJECTOR_SCREEN_CHROMIUM}" \
        --incognito \
        --kiosk \
        --no-first-run \
        --remote-debugging-port="${_PORT}" \
        --user-data-dir="$(mktemp -d)" \
        -- \
            "${PIJECTOR_SCREEN_DEFAULT_URL}" &
done && wait

