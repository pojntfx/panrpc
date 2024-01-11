#!/bin/bash

set -ex

mkdir -p ./out

echo "throughput" >./out/throughput-${SERIALIZER}.csv

export RESULTS=$(go run ./cmd/panrpc-example-tcp-throughput-client/ --serializer ${SERIALIZER})

IFS=$'\n'
for result in ${RESULTS}; do
    echo "${result}" >>./out/throughput-${SERIALIZER}.csv
done
