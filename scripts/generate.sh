#!/bin/bash
# This script is a wrapper for `generate_license.sh`

# Don't forget to export LICENSE_AUTH_TOKEN before executing

  ./generate-license.sh \
    --host http://nas:8443 \
    --agreement-id 1 \
    --email darrell.breeden@pharmalytica.io \
    --signing-key ~/.config/janus/signing-key.pub\
    --output ~/.config/janus/license.jwt

