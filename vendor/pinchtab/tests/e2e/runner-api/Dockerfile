FROM alpine:3.23

RUN apk add --no-cache \
    bash \
    curl \
    jq

WORKDIR /e2e

CMD ["/bin/bash", "-lc", "echo 'runner-api is executor-only; use go run ./tests/tools/runner e2e --suite ...' >&2; exit 2"]
