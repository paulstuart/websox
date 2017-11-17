#!/bin/bash

# you must set env.sh to contain the variables referenced below
. env.sh

m4 \
    -DUAA_URL=${uaa_url} \
    -DUAA_CLIENT_ID=${uaa_client_id} \
    -DUAA_CLIENT_SECRET=${uaa_client_secret} \
    manifest.yml.m4
