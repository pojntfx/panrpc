#!/bin/bash

set -ex

export DATA_TYPES=(int int8 int16 int32 rune int64 uint uint8 byte uint16 uint32 uint64 uintptr float32 float64 complex64 complex128 bool string array slice struct)

mkdir -p ./out

echo "data_type,serializer,runs" >./out/rps-${SERIALIZER}.csv
for data_type in "${DATA_TYPES[@]}"; do
    export RESULTS=$(go run ./cmd/dudirekta-example-tcp-rps-client/ --data-type ${data_type} --serializer ${SERIALIZER})

    IFS=$'\n'
    for result in ${RESULTS}; do
        echo "${data_type},${SERIALIZER},${result}" >>./out/rps-${SERIALIZER}.csv
    done
done
