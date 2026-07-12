#!/bin/bash
# Dummy NONMEM "compiler" for PsN resample-only setup.
#
# Used by the Janus K8s bootstrap saga (issues #192 / #193) as the nmfe for
# nm_version=dummy. It does NOT run an estimation: it exits immediately and
# produces no .lst/.ext, so PsN's run loop completes without fitting — leaving
# the resampled datasets + control files in m1/ intact for the host to fan out
# to the cluster.
#
# Usage (PsN invokes it like nmfe): dummy-nonmem.sh <model.mod> <output.lst> [opts...]
echo "dummy-nonmem: resample-only; skipping NONMEM estimation (no .lst/.ext produced)" >&2
exit 0
