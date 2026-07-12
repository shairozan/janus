# Janus Known Limitations

**Product:** Janus Desktop Application  
**Version:** [current version]  
**Last Updated:** October 26, 2025  
**Status:** Active Development

---

## Current Limitations

### Performance
- **Large Artifact Viewing**: Opening large byte stream artifacts (>100MB) in the UI can be slow due to JSON file storage and base64 decoding overhead
- **Single NONMEM Definition**: Only one NONMEM installation can be configured per session
- **HPC Scheduler Support**: Currently limited to SLURM; other schedulers require manual configuration

### Compatibility
- **NONMEM Version**: Requires NONMEM 7.x with valid license
- **Operating Systems**: 
  - Windows 11 (AMD64)
  - macOS (Intel/Apple Silicon)
  - Linux (AMD64/ARM64)
- **Hermes Integration**: Not yet available (planned for next release)

### Functionality
- **Project-Based Configuration**: Currently user-based configuration only; directory-based projects in development
- **Containerized Execution**: Requires Hermes integration (not yet released)
- **HPC Schedulers**: SGE support planned but not yet implemented
- **Audit Storage**: JSON file-based only; S3 plugin planned

---

## Workarounds

### For Project-Based Workflows
Users can switch between configurations by:
1. Modifying user configuration file
2. Restarting Janus
3. Alternative: Create separate OS user accounts for different projects

### For Large Artifacts
When working with large result files:
1. Use external tools for initial viewing
2. Use Janus for metadata and organization
3. Consider artifact size when configuring retention

### For Multiple NONMEM Versions
- Configure path to desired NONMEM version before session
- Restart Janus to switch NONMEM versions
- Alternative: Run multiple Janus instances (different ports)

---

## Roadmap

**Near-term (Next 1-2 Releases):**
1. Hermes integration (containerized execution)
2. Project-based configuration
3. Performance improvements for artifact viewing

**Medium-term (2-4 Releases):**
4. SGE scheduler support
5. S3 audit engine plugin
6. Multi-NONMEM configuration

**Long-term (Future):**
- Additional HPC scheduler support (PBS, LSF)
- Enhanced artifact storage options
- Collaborative features

---

## Known Issues

**None currently reported.**

Issues found during customer qualification will be documented here with:
- Issue description
- Severity
- Workaround (if available)
- Expected resolution timeline

---

## Reporting Issues

**Bug Reports:** bugs@pharmalytica.io  
**Feature Requests:** features@pharmalytica.io  
**Security Issues:** security@pharmalytica.io

Please include:
- Janus version
- Operating system
- NONMEM version
- Steps to reproduce (for bugs)
- Expected vs. actual behavior

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-10-26 | Initial known limitations document |

---

## Validation Impact

**For Customers Qualifying Janus:**

These limitations do not prevent qualification for intended use (pharmacometric modeling). However, customers should:

1. **Verify limitations are acceptable** for their specific use case
2. **Document known limitations** in site qualification report
3. **Establish workarounds** as needed for their workflow
4. **Track updates** via release notes for limitation resolutions

If any limitation is a blocker for your qualification, contact us at software+validation@pharmalytica.io to discuss timing or alternatives.