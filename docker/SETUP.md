# CI image setup

The CI images live in this repository's own GitHub Container Registry namespace:

```
ghcr.io/shairozan/janus-ci:ubuntu20
ghcr.io/shairozan/janus-ci:ubuntu22
ghcr.io/shairozan/janus-ci:ubuntu24
```

**There are no secrets to configure.** `.github/workflows/docker-build.yml`
authenticates with the `GITHUB_TOKEN` that GitHub issues to every workflow run,
so publishing needs no account, no access token, and nothing that can expire. A
fork publishes to its own namespace with the same zero setup.

## Building the images

The workflow runs weekly, and on demand:

**Actions → Build Docker Images → Run workflow**

It builds Ubuntu 20.04, 22.04 and 24.04 in parallel, smoke-tests each one (Go,
GCC, OpenGL, X11, mage, Xvfb), and pushes them.

## Make the packages public — the one manual step

**A newly published GHCR package is private, even when the repository is public.**
Nothing warns you. Everything that consumes the images then fails to pull them
with an authentication error that looks like a broken workflow rather than a
permissions setting.

After the first successful build, for each of the three packages:

1. Go to the repository → **Packages** (right-hand sidebar), or
   `https://github.com/users/shairozan/packages`
2. Open `janus-ci` → **Package settings**
3. Under **Danger Zone** → **Change visibility** → **Public**

This is needed once per package, not per build. If a workflow that used to work
suddenly cannot pull an image, check this first.

## Who consumes these images

| Consumer | Reference |
|---|---|
| `.github/workflows/test.yml` | `ubuntu22` |
| `.github/workflows/release.yml` | `ubuntu20`, `ubuntu22`, `ubuntu24` |
| `docker-compose.yml` | `latest-ubuntu24` |
| `docker/Dockerfile.dev` | `latest-ubuntu24` |
| `magefiles/docker.go` | `latest-ubuntu20/22/24` |

These all point at `ghcr.io/shairozan/janus-ci` explicitly rather than at the
current repository owner, so a fork's CI pulls the working upstream images
instead of needing to build its own first. Only the publishing workflow uses the
fork's own namespace.

## Building locally

You do not need the registry to work on the images:

```bash
docker build -f docker/Dockerfile.ubuntu22 -t janus-ci:ubuntu22 .
```

Or all three:

```bash
for version in 20 22 24; do
  docker build -f docker/Dockerfile.ubuntu${version} -t janus-ci:ubuntu${version} .
done
```

`mage docker:buildDev` builds the development image on top of the published
`latest-ubuntu24`, with the project's Go modules pre-fetched.

## Troubleshooting

**`denied` or `unauthorized` when pulling** — the package is almost certainly
still private. See the visibility step above.

**`denied: installation not allowed to Create organization package`** — the
workflow is missing `permissions: packages: write`.

**`invalid reference format`** — usually an uppercase character in the namespace.
GHCR requires lowercase image names, which is why the workflow lowercases
`GITHUB_REPOSITORY_OWNER`.

**Images look stale** — check that the weekly `Build Docker Images` run is
actually succeeding. It failed silently for two months once, and the only symptom
was Ubuntu security updates quietly not reaching the build environment.
