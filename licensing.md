# Janus Licensing Model

**Important Notice**: Janus is proprietary, closed-source software. All rights reserved.

## Overview

Janus operates under a commercial licensing model designed to provide sustainable funding for ongoing development while offering flexible deployment options for pharmaceutical organizations of all sizes.

## Licensing Structure

### Standard Commercial License

**Pricing**: $20 USD per user per month

**Features**:
- Full access to all Janus functionality
- NONMEM grid management across SLURM/SGE/TORQUE
- CFR 21 Part 11 compliance features
- Project management and audit trails
- Priority support and updates

**Install Limitations**:
- Up to 5 concurrent installations per licensed user
- Additional installations beyond the 5th will trigger license validation prompts
- Soft enforcement with usage monitoring for compliance

**Billing**:
- Monthly subscription model
- Automatic renewal unless cancelled
- Volume discounts available for organizations with 50+ users

### Enterprise Licensing

**Custom Pricing**: Available for organizations requiring:
- Unlimited installations per user
- On-premises license server deployment
- Custom integrations and development
- Dedicated support channels
- Service Level Agreements (SLAs)

## Partnership Models

### Metworx Integration Partnership

**Structure**: Revenue-sharing model with Metrum Research Group

**User Experience**:
- Metworx platform users receive complimentary Janus access
- Seamless integration with existing Metworx workflows
- No direct billing to end users

**Business Model**:
- Metrum pays $5 USD per unique active user per month
- Active user defined as: user who submits jobs or accesses Janus within a 30-day period
- Monthly reconciliation based on usage analytics
- Minimum commitment terms negotiable

**Benefits for Metrum**:
- Enhanced platform value proposition
- Reduced development overhead for grid management features
- Maintained control over user experience
- Competitive differentiation in the market

**Benefits for Janus**:
- Guaranteed revenue stream
- Market penetration in pharmaceutical sector
- Reduced customer acquisition costs
- Strategic partnership validation

## Technical Implementation

### License Validation

**Authentication Flow**:
1. OIDC-based license activation through setup wizard
2. Encrypted license file download during activation
3. Local license file validation (no network required)
4. Self-contained expiration and feature management

**License File Structure**:
```json
{
  "header": {
    "version": "1.0",
    "issuer": "janus-licensing",
    "issued_at": "2025-01-15T10:30:00Z",
    "expires_at": "2025-12-31T23:59:59Z"
  },
  "license": {
    "type": "metworx-partnership",
    "organization": "Customer Corporation",
    "organization_id": "metworx-customer-123",
    "user_limit": 100,
    "install_limit": 5,
    "features": ["audit", "projects", "grid-management"],
    "partnership_details": {
      "billing_model": "per_active_user",
      "rate_usd": 5.00,
      "metworx_customer_id": "customer-123"
    }
  },
  "metadata": {
    "machine_binding": "optional-hardware-fingerprint",
    "license_id": "uuid-license-identifier",
    "renewal_url": "https://licensing.janus-software.com/renew"
  },
  "signature": {
    "algorithm": "RSA-SHA256",
    "value": "base64-encoded-signature",
    "public_key_id": "janus-2025-primary"
  }
}
```

**Security Features**:
- RSA-2048 digital signatures for tamper detection
- AES-256 encryption for sensitive license data
- Public key rotation support via key identifiers
- Hardware binding for install limit enforcement (optional)
- Embedded expiration dates with cryptographic protection

**Airgapped Environment Support**:
- **Zero network dependency** after initial license acquisition
- License files can be transferred via secure media
- Manual license file placement in config directory
- Administrative license file distribution for organizations
- License renewal via file replacement (no connectivity required)

### Partnership Integration

**Metworx Secure Licensing Flow**:
- Customer authenticates via existing Metworx/Cognito infrastructure
- Metworx validates customer legitimacy and generates signed license request
- Janus licensing service validates Metworx signature and issues license file
- License file works offline and can be transferred to airgapped environments
- Cryptographic signatures prevent tampering and unauthorized use

**Technical Flow**:
1. **Authentication**: Customer logs into Metworx portal using existing Cognito pool
2. **License Request**: Metworx generates cryptographically signed license request
3. **Validation**: Janus licensing service validates Metworx signature using pre-shared public keys
4. **License Generation**: Upon validation, Janus issues encrypted license file
5. **Distribution**: License file downloaded by customer for local installation
6. **Verification**: Janus validates license file signature and expiration locally

**Security Model**:
- Pre-established trust relationship via public key exchange
- Metworx signs license requests with their private key
- Janus validates requests using Metworx public key
- Issued license files are encrypted and digitally signed by Janus
- No real-time connectivity required after license generation

**Usage Analytics** (Optional):
- Privacy-compliant user activity tracking (when connectivity available)
- Cached analytics for periodic upload when network accessible
- Monthly active user reporting for billing (batch processing)
- Performance metrics and adoption analytics
- Compliance reporting for both organizations
- **No impact on software functionality if analytics unavailable**

## Compliance and Legal

### Data Protection

**User Privacy**:
- Minimal data collection (user ID, organization, usage patterns)
- GDPR compliance for European users
- Data retention policies aligned with business needs
- User consent for telemetry collection

**Security**:
- End-to-end encryption for license communications
- Secure token storage with OS-level protection
- Regular security audits and penetration testing
- Incident response procedures

### Terms of Service

**Usage Rights**:
- Licensed software, not sold
- Restrictions on reverse engineering and redistribution
- Compliance with export control regulations
- Termination rights and data portability

**Support and Maintenance**:
- Regular updates and security patches
- Bug fixes and performance improvements
- Feature development roadmap transparency
- Migration assistance for version upgrades

## Market Positioning

### Value Proposition

**vs. Open Source Alternatives**:
- Professional support and guaranteed maintenance
- Enterprise-grade security and compliance features
- Integrated workflow optimization
- Dedicated development resources

**vs. Enterprise Competitors** (Pirana, etc.):
- **Airgapped environment support** - No network connectivity required
- Significantly lower cost of ownership ($20/month vs. enterprise pricing)
- No complex licensing schemes or audit risks
- **File-based licensing** - No license servers or periodic validation
- Modern, intuitive user interface
- Rapid deployment and configuration
- **Works in secure/classified environments**

### Target Markets

**Primary**:
- Mid-size pharmaceutical companies (50-500 employees)
- CROs and consulting organizations
- Academic institutions with commercial research
- Biotech companies with modeling needs

**Secondary**:
- Large pharma looking to reduce licensing costs
- Government research institutions
- Medical device companies
- International markets with cost sensitivity

## Revenue Projections

### Standard License Revenue

**Conservative Estimates** (Year 1):
- 100 paying users × $20/month = $24,000 annual recurring revenue
- 10% monthly growth rate
- 85% retention rate

**Growth Scenarios** (Year 2-3):
- Market penetration in pharmaceutical sector
- Feature expansion driving premium pricing
- Geographic expansion opportunities

### Partnership Revenue

**Metworx Integration** (Estimated):
- 200 active users × $5/month = $12,000 monthly revenue
- Potential for 1000+ users as platform scales
- Additional partnership opportunities with similar platforms

## Implementation Timeline

### Phase 1: Core Licensing (Months 1-3)
- OIDC integration implementation
- License validation service deployment
- Basic telemetry and usage tracking
- Standard commercial license launch

### Phase 2: Partnership Integration (Months 4-6)
- Metworx SSO integration development
- Partner billing and analytics systems
- Usage-based billing implementation
- Partnership agreement finalization

### Phase 3: Enterprise Features (Months 7-12)
- On-premises license server option
- Advanced compliance and audit features
- Custom integration development
- Enterprise support tier launch

## Risk Mitigation

### Technical Risks
- License service availability and redundancy
- Offline usage scenarios and grace periods
- Partner integration reliability
- Scale and performance considerations

### Business Risks
- Market acceptance of subscription model
- Competitive response from established players
- Partner relationship dependencies
- Regulatory compliance in multiple jurisdictions

### Mitigation Strategies
- Multiple deployment regions for license service
- Comprehensive offline support capabilities
- Diversified revenue streams beyond single partnerships
- Legal review and compliance automation

---

**Contact Information**:
- Licensing inquiries: licensing@janus-software.com
- Partnership discussions: partnerships@janus-software.com
- Technical integration: integration@janus-software.com

## Dependency License Compliance

### Overview

Janus has been designed with full consideration for license compatibility in commercial closed-source software development. All dependencies use permissive licenses that fully support our proprietary business model.

### License Analysis Summary

**✅ FULLY COMPATIBLE** - All dependencies cleared for commercial use without source disclosure requirements.

### Main Dependencies

**GUI & Application Framework:**
- **fyne.io/fyne/v2** - BSD-3-Clause License
  - Permissive GUI framework, commercial-friendly
  - No copyleft restrictions or source disclosure requirements

**CLI & Configuration:**
- **github.com/spf13/cobra** - Apache 2.0 License
  - CLI framework with explicit patent protections
  - Compatible with commercial closed-source software
- **github.com/spf13/viper** - MIT License
  - Configuration library with minimal restrictions
  - Allows commercial use without limitations

### License Categories

**Permissive Licenses Found:**
- **MIT License** - Highly permissive, minimal restrictions
- **BSD-2-Clause & BSD-3-Clause** - Permissive with attribution requirements
- **Apache 2.0** - Permissive with explicit patent grants

**No Copyleft Licenses:**
- ❌ **No GPL, LGPL, or AGPL licenses detected**
- ❌ **No licenses requiring source code disclosure**
- ❌ **No licenses prohibiting commercial use**

### Compliance Requirements

**Minimal Obligations:**
- Preserve copyright notices in software documentation
- Include attribution notices in about dialog or documentation
- No source code disclosure required
- No restrictions on $20/month licensing model

**Ongoing Monitoring:**
- License scanning implemented for new dependency additions
- Regular compliance reviews for dependency updates
- Documentation maintained for audit purposes

### Business Impact

This license analysis confirms that Janus can operate as proprietary closed-source software without any licensing conflicts. The dependency stack represents best practices for commercial Go development, using only business-friendly permissive licenses that support our commercial licensing model.

---

*This document is confidential and proprietary. Distribution is restricted to authorized personnel only.*