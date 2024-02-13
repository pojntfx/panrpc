#!/bin/bash

set -ex

mkdir -p ./out
cd go

echo "language,throughput" >../out/throughput-${SERIALIZER}.csv

export RESULTS=$(go run ./cmd/panrpc-example-tcp-throughput-client-cli/ --addr localhost:1337 --serializer ${SERIALIZER})

IFS=$'\n'
for result in ${RESULTS}; do
    echo "go,${result}" >>../out/throughput-${SERIALIZER}.csv
done

cd ../ts

export RESULTS=$(ADDR=localhost:1338 tsx ./bin/panrpc-example-tcp-throughput-client-cli.ts)

IFS=$'\n'
for result in ${RESULTS}; do
    echo "typescript,${result}" >>../out/throughput-${SERIALIZER}.csv
done
