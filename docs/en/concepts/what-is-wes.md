# What is WES

---

## One-Sentence Positioning

**WES is a third-generation blockchain that defines the verifiable computing paradigm, enabling decentralized intelligence in the AI era.**

> **Just as NVIDIA CUDA defined GPU general-purpose computing,**  
> **WES ISPC defines blockchain verifiable computing,**  
> **making production relations truly carry productive forces.**

---

## The Era's Proposition: Combining Production Relations and Productive Forces

In the AI era, we face a fundamental contradiction:

- **Production Relations** (Blockchain): Decentralized, immutable, transparent and trustworthy
- **Productive Forces** (AI): Intelligent, automated, efficient decision-making
- **Core Problem**: They are severely disconnected
  - **Traditional blockchains cannot run AI**: Deterministic consensus limitations, unable to support non-deterministic computing
  - **AI urgently needs blockchain**: Black-box decisions cannot be traced, critical scenarios lack auditability, data ownership is unclear, computing power monopoly costs are high

### Why Does AI Need Blockchain?

Although traditional blockchains cannot run AI, AI urgently needs blockchain to solve its fundamental problems:

**1. Trust Issues**
- **Black-box decisions**: Users don't know how AI makes decisions, cannot trust
- **Liability attribution**: When AI makes mistakes, who is responsible? Critical scenarios like healthcare and finance need clear liability attribution
- **Auditability**: Critical decisions need auditable, traceable records

**2. Data Issues**
- **Data ownership**: User data is used to train models, but users don't get rewards
- **Data privacy**: User data is collected and used by AI companies, users lose control
- **Data quality**: The source and quality of training data are difficult to verify

**3. Computing Power Issues**
- **Computing power monopoly**: A few large companies control computing resources
- **Computing power costs**: AI inference costs are high, small and medium enterprises cannot afford
- **Computing power waste**: Large amounts of computing power are wasted on repeated calculations

**4. Value Distribution Issues**
- **Value creators don't get rewards**: Data providers, model developers, computing power providers
- **Intermediaries extract value**: AI companies and cloud service providers extract most profits

**Blockchain can solve these problems**:
- ✅ **Verifiability**: ISPC can prove that AI used the correct model and input
- ✅ **Traceability**: Every AI decision has an on-chain record that can be traced
- ✅ **Data ownership**: Users can control their own data through blockchain
- ✅ **Distributed computing power**: Utilize dispersed computing resources, break monopoly
- ✅ **Redefine value distribution**: Through tokenization, let value creators directly receive rewards

**But traditional blockchains cannot run AI**, which is the essence of the contradiction.

### WES's Answer

**WES enables production relations to truly carry productive forces through the ISPC verifiable computing paradigm.**

---

## Strategic Positioning

### Third-Generation Blockchain

| Era | Representative | Definition | Applications | Limitations |
|-----|---------------|------------|--------------|-------------|
| **First Generation** | Bitcoin | Digital Currency | Value storage, payment | Can only transfer, cannot run business logic |
| **Second Generation** | Ethereum | Smart Contracts | DeFi, NFT, DAO | Cannot run AI, depends on external data/storage |
| **Third Generation** | **Weisyn** | **Verifiable Computing** | **AI, Enterprise Applications, All Complex Computing** | **Breakthrough deterministic consensus limitations** |

### ISPC: Not an Improvement, but a Paradigm

**ISPC (Intrinsic Self-Proving Computing)** is not a "feature improvement" to traditional blockchains, but a **paradigm innovation**:

- **Traditional Paradigm**: Deterministic consensus → All nodes repeat execution → Can only do simple computing
- **ISPC Paradigm**: Verifiability consensus → Single execution + multi-point verification → Supports complex computing like AI

**Analogy**:
- CUDA to GPU = Opens the era of general-purpose computing
- ISPC to blockchain = Opens the era of verifiable computing

---

## Core Values

### Core Differentiation: AI Native Capability

**WES is the first blockchain platform that truly supports AI running on-chain.**

Why can't traditional blockchains run AI?
- ❌ **Deterministic consensus requirement**: Same input must produce same output → AI inference is non-deterministic
- ❌ **Repeated execution limitation**: All nodes repeat execution → AI models are too large, computing is too expensive
- ❌ **Cannot integrate external**: Needs oracle to feed data → Real-time data cannot be obtained

How does WES ISPC break through?
- ✅ **Verifiability consensus**: Verify ZK Proof, don't require same result → Supports non-deterministic computing
- ✅ **Single execution + multi-point verification**: Only one node executes AI inference → Cost reduced by 99%
- ✅ **Controlled external interaction**: HostABI provides verifiable external integration → Trustworthy real-time data acquisition

**AI Native Application Scenarios**:
- **On-chain AI Smart Contracts**: Directly call AI inference within contracts
- **AI-driven DeFi**: Intelligent investment advice, risk assessment
- **Decentralized AI Services**: AI model inference as mining
- **AI + Blockchain Fusion Innovation**: Trustworthy AI, AI DAO

> **Born for AI, but not limited to AI** — Just as NVIDIA chips support various computing, ISPC also supports traditional enterprise applications.

### Foundation: Enterprise Application Support

**WES is the first blockchain platform that truly supports enterprise applications.**

Why can't traditional blockchains do enterprise applications?
- ❌ **External side effects problem**: 50 nodes = 50 database operations = database crash
- ❌ **Atomicity limitation**: Only supports single transactions, cannot support long transaction business processes
- ❌ **High integration costs**: Requires large-scale modification of traditional business systems

How does WES ISPC break through?
- ✅ **Single execution + multi-point verification**: Only one node executes business logic, other nodes verify ZK Proof
- ✅ **Atomic container**: Entire business process executes within one atomic boundary
- ✅ **Controlled external interaction**: Traditional business systems don't need modification, can seamlessly integrate

**Core Values**:
1. **Breakthrough external side effects bottleneck** - Enable enterprise applications that need multi-party trusted collaboration and external system integration to go on-chain
2. **Support enterprise-level long transactions** - Enable complex business processes to execute atomically
3. **Zero modification cost integration** - Enable traditional business systems to seamlessly integrate

### Byproduct: Flexible Fee Mechanism

**Fee as Incentive (Core)** - Transaction fees aggregated as miner incentives, zero inflation model  
**Multi-Token Payment** - Users can pay with multiple tokens, no need to hold specific platform coins  
**ISPC Cost Optimization** - Single execution significantly reduces computing costs, fees are lower  
**Business-Level Sponsorship Pool (Optional)** - Business parties can reward miners through sponsorship pools and their own tokens, creating a "user almost fee-free" experience

---

## System Architecture

### Three-Layer Classic Model

WES adopts a classic three-layer architecture model:

```
Interaction Layer → Computation Layer → Ledger Layer
```

**First Layer: Interaction Layer**
- Define operation inputs: UTXO references (consumable/reference), parameters, resource references
- Define operation outputs: Asset/Resource/State three output types

**Second Layer: Computation Layer**
- **ISPC is the core of this layer**: Execute computation and automatically generate verifiable proofs
- Support WASM smart contracts and ONNX AI model execution
- Realize paradigm breakthrough of single execution + multi-point verification

**Third Layer: Ledger Layer**
- **EUTXO**: Manage state sets of Asset/Resource/State three-layer outputs
- **URES**: Content-addressable storage, manage WASM/ONNX/files and other resources
- **Block Ledger**: Immutable transaction history records

**Consensus Guarantee (Across All Layers)**:
- **PoW+XOR**: Hybrid consensus mechanism, guarantees security and consistency of three-layer coordination

### Four Core Innovations

| Innovation Feature | Positioning | Core Value |
|-------------------|-------------|------------|
| **[ISPC Intrinsic Self-Proving Computing](./ispc.md)** | Computing execution layer innovation | Execution-as-proof mechanism, single execution + multi-point verification, zero-knowledge proof integration |
| **[EUTXO Extended Model](./eutxo.md)** | State layer innovation | Three-layer output architecture (Asset/Resource/State) + reference without consumption mode |
| **[URES Unified Resource Management](./ures.md)** | Resource management layer innovation | Unified management of static and executable resources, implementing blockchain file system |
| **[PoW+XOR Distance Selection Consensus](./consensus-pow-xor.md)** | Consensus layer innovation | Proof of Work + XOR distance selection, microsecond confirmation + complete decentralization |

---

## Typical Application Scenarios

### AI Native Scenarios (Core Differentiation)

**On-chain AI Smart Contracts**:
- AI-driven DeFi: Intelligent investment advice, risk assessment, automated trading strategies
- AI + NFT: Dynamic NFT generation, AI art creation, intelligent copyright protection
- AI-driven DAO: Intelligent proposal evaluation, automated governance decisions
- Trustworthy AI Services: Decentralized AI inference, AI model marketplace, AI as mining

**AI + Blockchain Fusion Innovation**:
- On-chain AI Agents: Autonomous AI agents that execute tasks
- Decentralized AI Training: Distributed model training and incentives
- AI-driven Prediction Markets: Intelligent analysis and prediction

### Enterprise Application Scenarios (Foundation)

**Multi-Party Trusted Collaboration**:
- Supply Chain Management: Full-chain traceability from raw materials to finished products
- Cross-Institution Clearing: Inter-bank clearing and settlement
- Production Management: Production planning, execution, monitoring, traceability
- Compliance Auditing: Complete business trajectory and evidence chain

**External System Integration**:
- Order Processing: Order creation → Inventory deduction → Payment processing → Logistics arrangement → Invoice generation → Financial accounting
- ERP Integration: Seamless integration with Enterprise Resource Planning systems
- CRM Integration: Trustworthy records of Customer Relationship Management
- Financial System Integration: Automated financial processing and accounting

---

## Why Choose WES?

### Value for Different Roles

| For Users | For Developers | For Enterprises |
|-----------|----------------|-----------------|
| **Zero-cost usage** | **Lower learning costs** | **Enterprise-grade reliability** |
| **Fast confirmation** | **Multi-language support** | **Complete auditing** |
| **Security guarantee** | **Rich toolchain** | **Transparent costs** |
| **AI Native applications** | **Paradigm innovation opportunities** | **Compliance ready** |

### Core Competitiveness

**Paradigm Innovation**:
- ISPC defines the blockchain verifiable computing paradigm
- Just as CUDA opened the GPU general-purpose computing era
- Not an improvement, but a redefinition

**Core Differentiation**:
- AI Native capability: First blockchain that supports AI running on-chain
- Born for AI, but not limited to AI

**Foundation**:
- First blockchain platform that truly supports enterprise applications
- Breakthrough external side effects bottleneck, support long transactions, zero modification integration

---

## Next Steps

### Quick Start

- **[Installation Guide](../getting-started/installation.md)** - Prepare development environment
- **[Quick Start](../getting-started/quickstart-local.md)** - Get up and running in 5 minutes

### Deep Dive

- **[Architecture Overview](./architecture-overview.md)** - Understand system architecture
- **[ISPC Technical Details](./ispc.md)** - Understand core computing paradigm
- **[EUTXO Model](./eutxo.md)** - Understand state model

### Development Related

- **[API Reference](../reference/api/)** - Complete API reference
- **[Tutorials](../tutorials/)** - End-to-end learning path

---

**WES: Making production relations truly carry productive forces.**

