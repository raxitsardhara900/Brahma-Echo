#!/bin/bash
# Common API/curl E2E entrypoint.

HELPERS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${HELPERS_DIR}/base.sh"

E2E_REF_JSON_VAR="RESULT"

source "${HELPERS_DIR}/api-assertions.sh"
source "${HELPERS_DIR}/api-http.sh"
