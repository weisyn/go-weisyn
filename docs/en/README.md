# WES Documentation Center

Welcome to the WES (Weisyn) Documentation Center!

**WES defines the verifiable computing paradigm for blockchain, enabling decentralized intelligence in the AI era.**

---

## 🎯 About WES

WES is a **third-generation blockchain** that breaks through the deterministic consensus limitations of traditional blockchains through **ISPC (Intrinsic Self-Proving Computing)**, a verifiable computing paradigm.

### Core Innovations

| Innovation Feature | Positioning | Core Value |
|-------------------|-------------|------------|
| **ISPC Intrinsic Self-Proving Computing** | Computing execution layer innovation | Single execution + multi-point verification, supporting complex computations like AI on-chain |
| **EUTXO Extended Model** | State layer innovation | Three-layer output architecture (Asset/Resource/State) + reference without consumption mode |
| **URES Unified Resource Management** | Resource management layer innovation | Content-addressable storage, unified management of contracts/AI models/files |
| **PoW+XOR Distance Selection Consensus** | Consensus layer innovation | Proof of Work + XOR distance selection, high-performance consensus |

### Core Values

- ✅ **AI Native**: The only blockchain in the industry that supports on-chain AI model inference
- ✅ **Enterprise Application Support**: Supports long transactions, external system integration, truly carrying enterprise-level business
- ✅ **User Gas-Free Experience**: Uses CU (Compute Units) as internal computing power measurement, users don't need to understand

---

## 🚀 Quick Start

### I'm new, where should I start?

**3 steps to get started:**

1. **Learn about WES** → [What is WES](./concepts/what-is-wes.md) - Understand WES positioning and value (10 minutes)
2. **Quick Experience** → [Local Quick Start](./getting-started/quickstart-local.md) - Get up and running in 5 minutes
3. **Start Development** → [API Reference](./reference/api/) - Begin integration development

---

## 🧭 Entry Points (Division of Labor with Repository README)

- **Repository Root [`README.md`](../../../README.md)**: Product/vision entry (why we're building, what problems we solve, quick experience), suitable for readers learning about WES for the first time.
- **This Documentation Center `docs/en/`**: Systematic learning and usage entry (Getting Started → Concepts → Tutorials → How-to Guides → Reference), targeting developers/architects/operators/contributors.
- **Internal R&D Knowledge Base `_dev/`**: Protocol specifications and design documents (Source of Truth), targeting implementers; public documentation only summarizes key contracts and boundaries, without copying all specification text.

---

## 👥 Navigation by Role

### 👨‍💻 Developers

**Quick Start**
- [Installation Guide](./getting-started/installation.md) → [Local Quick Start](./getting-started/quickstart-local.md) → [First Transaction](./getting-started/first-transaction.md)

**Deep Dive**
- [Core Concepts](./concepts/) → [Contract Development Tutorial](./tutorials/contracts/) → [API Reference](./reference/api/)

**Learning Path**: Understand WES → Deploy Node → Write Contracts → Integrate Applications

---

### 🏗️ Architects

**Understand System Architecture**
- [Architecture Overview](./concepts/architecture-overview.md) → [Core Concepts](./concepts/) → [ISPC Technical Details](./concepts/ispc.md)

**Deep Dive**
- [EUTXO Model](./concepts/eutxo.md) → [URES Resource Management](./concepts/ures.md) → [PoW+XOR Consensus](./concepts/consensus-pow-xor.md)

**Learning Path**: System Architecture → Core Innovations → Technical Implementation

---

### 💼 Decision Makers / Product Managers

**Understand Project Value**
- [What is WES](./concepts/what-is-wes.md) → [FAQ](./getting-started/faq.md)

**Learning Path**: Strategic Positioning → Competitive Analysis → Application Scenarios

---

### 🔧 Operators

**Deployment and Operations**
- [Installation Guide](./getting-started/installation.md) → [Deployment Guide](./how-to/deploy/) → [Troubleshooting](./how-to/troubleshoot/)

**Learning Path**: Environment Deployment → Troubleshooting → Performance Tuning

---

## 📚 Documentation Map

```
docs/en/
├── getting-started/           # 🚀 Getting Started
│   ├── installation.md        # Installation Guide
│   ├── quickstart-local.md    # Local Quick Start
│   ├── quickstart-docker.md   # Docker Quick Start
│   ├── first-transaction.md   # First Transaction
│   └── faq.md                 # FAQ
│
├── concepts/                  # 💡 Core Concepts
│   ├── what-is-wes.md         # What is WES
│   ├── architecture-overview.md # Architecture Overview
│   ├── ispc.md                # ISPC Intrinsic Self-Proving Computing
│   ├── eutxo.md               # EUTXO Extended Model
│   ├── ures.md                # URES Unified Resource Management
│   ├── consensus-pow-xor.md   # PoW+XOR Consensus
│   ├── transaction.md         # Transaction Model
│   ├── block.md               # Block Model
│   ├── chain.md               # Chain Model
│   ├── network-and-topology.md # Network and Topology
│   ├── data-persistence.md    # Data Persistence
│   ├── privacy-and-proof.md   # Privacy and Proof
│   ├── governance-and-compliance.md # Governance and Compliance
│   └── glossary.md            # Glossary
│
├── tutorials/                 # 📖 Tutorials
│   ├── contracts/             # Contract Development Tutorial
│   ├── ispc/                  # ISPC Tutorial
│   ├── deployment/            # Deployment Tutorial
│   └── scenarios/             # Scenario Practices
│
├── how-to/                    # 🔧 How-to Guides
│   ├── operate/               # Operations
│   ├── deploy/                # Deployment Operations
│   ├── configure/             # Configuration Guide
│   ├── integrate/             # Integration Guide
│   ├── secure/                # Security Operations
│   └── troubleshoot/          # Troubleshooting
│
├── reference/                 # 📋 Reference Documentation
│   ├── api/                   # API Reference
│   ├── cli/                   # CLI Reference
│   ├── config/                # Configuration Reference
│   ├── schema/                # Data Formats
│   ├── error-codes.md         # Error Code Reference
│   └── ports.md               # Port Specifications
│
├── contributing/              # 🤝 Contributing Guide
│   ├── development-setup.md   # Development Environment Setup
│   ├── code-style.md          # Code Standards
│   ├── docs-style.md          # Documentation Standards
│   └── design-docs.md         # Design Document Guide
│
└── support/                   # 📞 Support
    ├── compatibility.md       # Compatibility Policy
    ├── support-policy.md      # Support Policy
    └── releases.md            # Version Releases
```

---

## 🎯 Find by Task

### I want to learn about the project

- [What is WES](./concepts/what-is-wes.md) - Product overview: positioning, value, features
- [Architecture Overview](./concepts/architecture-overview.md) - System architecture overview
- [Glossary](./concepts/glossary.md) - Term definitions

### I want to start developing

- [Installation Guide](./getting-started/installation.md) - Environment setup
- [Quick Start](./getting-started/quickstart-local.md) - Get started in 5 minutes
- [API Reference](./reference/api/) - Interface documentation

### I want to learn contract development

- [Contract Introduction](./tutorials/contracts/) - Contract development tutorial
- [ISPC Tutorial](./tutorials/ispc/) - End-to-end ISPC tutorial

### I want to deploy and operate

- [Deployment Guide](./how-to/deploy/) - Deployment operation guide
- [Configuration Guide](./how-to/configure/) - Configuration instructions
- [Troubleshooting](./how-to/troubleshoot/) - Problem troubleshooting

### I want to contribute code

- [Development Environment Setup](./contributing/development-setup.md) - Environment setup
- [Code Standards](./contributing/code-style.md) - Coding standards
- [Design Document Guide](./contributing/design-docs.md) - How to read design documents in `_dev/`

---

## ❓ FAQ

### Q: Is the documentation up to date?

A: Documentation is continuously updated. Please check the update date at the top of the document, or submit an Issue to inquire.

### Q: What should I do if I can't find the information I need?

A:
1. Use browser search (Ctrl+F / Cmd+F)
2. Check [FAQ](./getting-started/faq.md)
3. Submit an Issue to tell us what's missing

### Q: How do I contribute code?

A: Pull Requests are welcome! Please check the [Contributing Guide](./contributing/development-setup.md).

---

## 🔗 Related Resources

- **Internal Design Documents**: [`_dev/`](../../_dev/) - Internal knowledge base for core R&D and architects
- **Issue Reporting**: GitHub Issues
- **Community Discussion**: GitHub Discussions

---

**WES: Making production relations truly carry productive forces.** 🚀

