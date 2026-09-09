# Janus Product Description

**Product Name:** Janus  
**Product Type:** Desktop Application  
**GAMP Category:** Category 4 (Configured Product)  
**Current Version:** [version]  
**Regulatory Classification:** Non-medical device software tool

---

## Purpose

Janus is a desktop workbench for pharmacometric modeling that provides:
- Project and model organization
- NONMEM execution management across multiple environments:
  - Local execution
  - HPC schedulers (SLURM)
  - Containerized execution (via Hermes)
- Integrated execution tracking with colocated file system storage:
  - STDOUT/STDERR capture
  - Artifact collection (output files, diagnostics, plots)
  - Execution metadata (command, timestamp, exit status)
  - [Optional] Hermes execution events
- Workflow automation and result visualization

---

## Architecture

**Platform:** Cross-platform desktop application
- Windows 11 (AMD64)
- macOS (Intel/Apple Silicon)
- Linux (AMD64/ARM64)

**Storage:** Local file system (user's computer)
- Models and data files managed by user
- Execution logs stored alongside model files
- No central database or cloud storage

**Execution:** Command-line interface to NONMEM
- Direct local execution
- HPC job submission (SLURM)
- [Optional] Containerized execution via Hermes proxy

**Integration:** Optional connection to Hermes execution proxy for containerized, isolated NONMEM runs

---

## What Janus Does

**Model Management:**
- Organize NONMEM models and related files
- Track model versions and iterations
- Manage input data and control streams

**Execution Management:**
- Execute NONMEM models locally or on HPC
- Monitor execution progress
- Capture execution logs and artifacts
- Collect specified output files

**Results Organization:**
- Consolidate execution outputs
- Provide visualization and diagnostics
- Generate execution journals for documentation
- Support result comparison across runs

**Workflow Support:**
- Facilitate iterative modeling workflows
- Support documentation and reporting
- Enable reproducibility through execution tracking

**Evidence Generation:**
- Produce a cryptographically signed execution record for each run (RSA-SHA256 over the full record)
- Bind each record to a signer identity (public key, SHA256 fingerprint, licensed user email, signing timestamp)
- Capture container provenance for containerized executions (image URI, tag, SHA digest)
- Allow any party to verify a record's integrity and attribution without access to the signer's license

---

## What Janus Does NOT Do

**Janus is a tool, not a system of record:**

- **Does NOT store GxP records** - All files remain on user's file system under user's control
- **Does NOT implement Part 11 electronic signatures** - Record signing establishes integrity and attribution; it is not an electronic signature ceremony under §11.100/§11.200 (no signing-intent capture, no re-authentication, no signature meaning)
- **Does NOT maintain a record-modification audit trail** - Janus records *executions*, not subsequent changes to records; retention and versioning remain the user's responsibility
- **Does NOT control access to records** - Authentication, authorization, and record retention are provided by the operating environment and the customer's QMS
- **Does NOT execute NONMEM directly** - Calls user's installed NONMEM
- **Does NOT validate models** - Scientific validation is user's responsibility
- **Does NOT replace organizational QMS** - Users must maintain records per their procedures

**Execution records vs. Part 11 audit trails:**
Janus produces a signed, tamper-evident record of *what ran, under which container, invoked by whom, and when*. That is evidence a customer can rely on when meeting their own Part 11 and predicate-rule obligations — but it is not, by itself, a Part 11 audit trail, which must also capture operator changes to records over time. Users remain responsible for maintaining audit trails per their QMS requirements.

---

## System of Record

**The user's file system and organizational QMS are the systems of record.**

Janus operates on files managed by the user:
- Model files (.ctl, .mod)
- Data files (.csv, etc.)
- Output files (.lst, .ext, etc.)
- Execution logs (.json)

Users are responsible for:
- File backup and retention per their procedures
- Version control (e.g., Git) if required
- Audit trail maintenance per their QMS
- Record retention per applicable regulations

---

## Intended Use in GxP Environments

When used in GxP environments, Janus serves as a **qualified tool** in validated workflows:

**Customer Responsibilities:**
1. Qualify Janus for intended use (IQ/OQ/PQ)
2. Maintain GxP records in validated systems
3. Implement audit trail per Part 11 requirements (if applicable)
4. Train users and document competency
5. Control version and manage changes

**Janus Provides:**
- Validation support package (requirements, test results)
- Customer qualification guide (IQ/OQ/PQ templates)
- Technical support during qualification
- Release notes and change documentation

See [Customer Qualification Guide](customer-qualification-guide.md) for details.

---

## Regulatory Position

**GAMP Category 4:** Configured product (commercial software with user configuration)

**Part 11 Applicability:** Janus is not a Part 11 system; it is a tool that produces Part 11-relevant evidence.

No software is "Part 11 compliant" in isolation — compliance is a property of the system together with the
customer's procedures, controls, and organization. Janus does not attempt to be the customer's Part 11
system. What it does is generate signed, attributable, reproducible execution evidence that the customer
can rely on inside *their* validated environment.

| Concern | Janus | Customer |
|---|---|---|
| Record integrity (tamper evidence) | Provides — RSA-SHA256 signature over the full run record | Verifies and retains |
| Attribution (who ran it) | Provides — signer public key, fingerprint, licensed user email, signed-at timestamp | Maps licensed identity to their user directory |
| Reproducibility (what it ran on) | Provides — container image URI, tag, and SHA digest | Retains the image or registry reference |
| Electronic signatures (§11.100, §11.200) | Not provided | Provides, if the predicate rule requires them |
| Record-modification audit trail (§11.10(e)) | Not provided | Provides via QMS / validated system |
| Access control and retention (§11.10(c), (d)) | Not provided | Provides via operating environment and QMS |

**Comparison:** Similar regulatory position to:
- Certara Pirana (modeling workbench)
- Microsoft Excel (calculations tool)
- MATLAB (computational environment)
- R/Python (statistical computing)

...with one distinction worth stating plainly: unlike a general-purpose computational environment, Janus
emits a signed execution record by design, so the customer does not have to construct that evidence by
hand or by script.

**Customer Validation:** Customers qualify Janus as a tool used in GxP workflows, not as a Part 11 system.
Per GAMP 5, customers may leverage supplier evidence (see the Validation Support Package) rather than
re-deriving qualification themselves.

---

## Integration: Hermes Execution Proxy

Janus can optionally integrate with Hermes for containerized execution:

**Benefits:**
- Isolated execution environment
- Reproducible computational environment
- Clean workspace management
- Enhanced execution traceability

**When using Hermes:**
- Both Janus and Hermes should be qualified together
- Hermes execution events can be captured in Janus execution logs
- See Hermes documentation for integration details

**Status:** Hermes integration planned for upcoming release

---

## Document Control

**Document Version:** 1.0  
**Date:** October 26, 2025  
**Author:** Janus maintainers  

**Related Documents:**
- INTENDED_USE.md - Detailed intended use statement
- KNOWN_LIMITATIONS.md - Current limitations and workarounds
- customer-qualification-guide.md - Site qualification procedures
- Release Notes - Version-specific changes

---

## Contact

Janus is community-supported open source. There is no vendor support desk.

**Questions and bug reports:** https://github.com/shairozan/janus/issues  
**Security issues:** see [SECURITY.md](../../SECURITY.md)