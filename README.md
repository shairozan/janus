# Janus: NONMEM Grid Management Tool

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An open-source alternative to Certara Pirana, built with Go + Fyne.io, with
pluggable orchestrator backends and a cryptographically signed execution record
suited to CFR 21 Part 11 environments.

# A different methodology
Janus is built around a different core tenet than most NONMEM orchestrators. It always felt ugly to "create new directory, run there, leave detritus behind". While the run matters, and its artifacts do, Janus prioritizes a clean _current representation_ of the model and its data as opposed to a cluttered one. All the artifacts from a run are stored and encoded into the actual run log itself. They're always at arms length to view, download and extract. Just not always in your face. 

# The Runlog
When you setup Janus, you'll setup a key with your local trust store in your operating system. This will be used for _each run_ to sign the run such that anyone on the outside can verify whether the content has been tampered with. The run log itself is also useful for comparing objective function values, and visualizing them across multiple runs (Although without the R dependencies)

# Hermes orchestration
Janus is also capable of running NONMEM directly in a container via docker or Kubernetes (Fan out for psn bootstrap operations) via the [Hermes](https://github.com/shairozan/hermes) command proxy. Janus will list any containers that match the OCI spec, allowing you just worry about running hte job. Janus handles making sure the NONMEM license gets where it needs to, what command to issue to the container and *extracts all files you specify*. This should isolate the container for validation efforts without much caring about anything outside of the container.

# AI Forward
Whether running from the desktop application or headless as a daemon, an MCP server is exposed to allow practically all functionality (including reasoning over the run log or running new jobs) via an agent directly. 


# NONMEM Lexer
A go-based lexer for the NONMEM control stream was written for this project to allow for a natively syntax highlighted editor. Eventually I'll pull this up into the top level of the repo so that other things can consume it as a library. 

---

*"Less hype, more function" - build something that works, adoption will follow.*
