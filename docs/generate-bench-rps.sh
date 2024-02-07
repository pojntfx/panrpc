#!/bin/bash

set -ex

export DATA_TYPES=(int int8 int16 int32 rune int64 uint uint8 byte uint16 uint32 uint64 uintptr float32 float64 complex64 complex128 bool string array slice struct)

mkdir -p ./out
cd go

echo "language,data_type,runs" >../out/rps-${SERIALIZER}.csv
for data_type in "${DATA_TYPES[@]}"; do
    export RESULTS=$(go run ./cmd/panrpc-example-tcp-rps-client/ --addr localhost:1337 --data-type ${data_type} --serializer ${SERIALIZER})

    IFS=$'\n'
    for result in ${RESULTS}; do
        echo "go,${data_type},${result}" >>../out/rps-${SERIALIZER}.csv
    done
done

cd ../ts
for data_type in "${DATA_TYPES[@]}"; do
    export RESULTS=$(ADDR=localhost:1338 DATA_TYPE=${data_type} tsx ./bin/panrpc-example-tcp-rps-client.ts)

    IFS=$'\n'
    for result in ${RESULTS}; do
        echo "typescript,${data_type},${result}" >>../out/rps-${SERIALIZER}.csv
    done
done
