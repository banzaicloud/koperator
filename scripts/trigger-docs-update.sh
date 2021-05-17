#!/bin/bash

set -euf

PROJECT_SLUG='gh/banzaicloud/kafka-operator-docs'
RELEASE_TAG="$1"

function main()
{
    curl \
        -u "${CIRCLE_TOKEN}:" \
        -X POST \
        --header "Content-Type: application/json" \
        -d "{
            \"branch\": \"master\",
            \"parameters\": {
                \"remote-trigger\": true,
                \"project\": \"kafka-operator\",
                \"release-tag\": \"${RELEASE_TAG}\",
                \"build-dir\": \"docs/types/\"
            }
        }" "https://circleci.com/api/v2/project/${PROJECT_SLUG}/pipeline"
}

main "$@"
