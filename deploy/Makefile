# Janus license server — GKE deploy helpers.
#
# Secrets are SOPS/AGE-encrypted and COMMITTED to the repo. KSOPS decrypts them inside
# `kustomize build`, so the whole stack (including secrets) deploys with a single
# `kustomize build | kubectl apply -f -`.
#
# Two components live in the janus-license namespace, each with its own overlay:
#   OVERLAY=license-server/overlays/gke   the Go license-server (API)   image janus-license-server
#   OVERLAY=portal/overlays/gke           the Next.js portal   (UI)     image janus-portal
# Deploy each with its own `make apply OVERLAY=<path>`.
#
# Requirements: kustomize (standalone, not `kubectl kustomize`), the ksops exec plugin
# on PATH, sops, and the AGE private key available to sops (SOPS_AGE_KEY_FILE or
# ~/.config/sops/age/keys.txt).

OVERLAY    ?= license-server/overlays/gke
SECRET_DIR := license-server/secrets
SECRETS    := $(wildcard $(SECRET_DIR)/*.sops.yaml)
KUSTOMIZE  ?= kustomize
KFLAGS     := --enable-alpha-plugins --enable-exec

# VERSION optionally pins the component's image to a specific tag at apply time, e.g.
# `make apply OVERLAY=license-server/overlays/gke VERSION=0.2.4`. The overlay is retagged
# for this one build and its kustomization.yaml is restored byte-for-byte afterwards, so
# git stays on :latest and nothing is committed. The image is derived from OVERLAY, since
# each overlay pins exactly one image. Note: a pinned build restores the overlay from a
# backup on exit — commit any manual overlay edits before using VERSION.
#
# The git tag `vX.Y.Z` publishes image tag `X.Y.Z` (docker/metadata-action strips the v),
# so pass VERSION without the leading v.
VERSION ?=

ifneq (,$(findstring license-server,$(OVERLAY)))
IMAGE_NAME    := janus-license-server
IMAGE_NEWNAME := ghcr.io/pharmalytica/janus/license-server
else ifneq (,$(findstring portal,$(OVERLAY)))
IMAGE_NAME    := janus-portal
IMAGE_NEWNAME := ghcr.io/pharmalytica/janus/portal
endif

# PIN retags the overlay in place for a single build when VERSION is set, backing the file
# up first and restoring it on shell EXIT (kustomize edit would otherwise strip comments and
# reformat). Make collapses the backslash-newlines into one shell line so the trap spans the
# whole build/apply. No-op when VERSION is empty.
PIN = if [ -n "$(VERSION)" ]; then \
	test -n "$(IMAGE_NAME)" || { echo "make: VERSION=$(VERSION) set but OVERLAY=$(OVERLAY) maps to no image" >&2; exit 2; }; \
	echo ">> pinning $(IMAGE_NAME) => $(IMAGE_NEWNAME):$(VERSION) for this build (overlay restored on exit)" >&2; \
	__bak=$$(mktemp) && cp "$(OVERLAY)/kustomization.yaml" "$$__bak" && \
	trap 'mv -f "$$__bak" "$(OVERLAY)/kustomization.yaml"' EXIT && \
	( cd "$(OVERLAY)" && $(KUSTOMIZE) edit set image $(IMAGE_NAME)=$(IMAGE_NEWNAME):$(VERSION) ); \
	fi

.PHONY: help build apply diff delete secrets-edit secrets-view

help:
	@echo "Targets:"
	@echo "  build                 kustomize build the overlay (decrypts secrets via KSOPS)"
	@echo "  apply                 build | kubectl apply -f -   (deploys everything)"
	@echo "  diff                  build | kubectl diff -f -"
	@echo "  delete                build | kubectl delete -f -  (keeps namespace/PVC)"
	@echo "  secrets-edit F=path   Edit an encrypted secret in place (sops)"
	@echo "  secrets-view          Decrypt and print all *.sops.yaml secrets"
	@echo ""
	@echo "Vars:"
	@echo "  OVERLAY=<path>        component overlay (default license-server/overlays/gke;"
	@echo "                        portal is portal/overlays/gke)"
	@echo "  VERSION=<tag>         pin this component's image to <tag> for the run, e.g. 0.2.4"
	@echo "                        (overlay stays on :latest in git). Deploy v0.2.4:"
	@echo "                          make apply OVERLAY=license-server/overlays/gke VERSION=0.2.4"
	@echo "                          make apply OVERLAY=portal/overlays/gke         VERSION=0.2.4"
	@echo ""
	@echo "Create a secret from a template, e.g.:"
	@echo "  cp $(SECRET_DIR)/license-secrets.example.yaml $(SECRET_DIR)/license-secrets.sops.yaml"
	@echo "  \$$EDITOR $(SECRET_DIR)/license-secrets.sops.yaml && sops -e -i $(SECRET_DIR)/license-secrets.sops.yaml"

# Full render, including decrypted secrets. (Prints secret material to stdout.)
build:
	@$(PIN) && $(KUSTOMIZE) build $(KFLAGS) $(OVERLAY)

apply:
	@$(PIN) && $(KUSTOMIZE) build $(KFLAGS) $(OVERLAY) | kubectl apply -f -

diff:
	@$(PIN) && $(KUSTOMIZE) build $(KFLAGS) $(OVERLAY) | kubectl diff -f -

# Removes the workload but intentionally leaves the namespace and the Postgres PVC.
delete:
	@$(PIN) && $(KUSTOMIZE) build $(KFLAGS) $(OVERLAY) | kubectl delete -f - --ignore-not-found

secrets-edit:
	@test -n "$(F)" || { echo "usage: make secrets-edit F=$(SECRET_DIR)/<name>.sops.yaml"; exit 1; }
	sops $(F)

secrets-view:
	@for f in $(SECRETS); do echo "# --- $$f"; sops --decrypt $$f; echo; done
