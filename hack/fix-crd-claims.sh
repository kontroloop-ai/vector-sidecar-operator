#!/bin/bash
# Fix CRD validation issues with resource claims x-kubernetes-map-type
# This is a workaround for controller-gen not properly handling the claims field
set -e

CRD_FILE="config/crd/bases/observability.amitde789696.io_vectorsidecars.yaml"

if [ ! -f "$CRD_FILE" ]; then
    echo "CRD file not found: $CRD_FILE"
    exit 1
fi

# Run the Python fix script
python3 hack/fix-crd-claims.py
