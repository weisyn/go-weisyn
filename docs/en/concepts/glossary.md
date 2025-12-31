# Glossary

---

This document defines core terms used in the WES system.

---

## A

### Aggregator (XOR Distance Selection Node)
In PoW+XOR consensus, the node role responsible for collecting candidate blocks and performing XOR distance selection. Dynamically decides whether to act as an aggregator node based on K-bucket distance, performs XOR distance calculation on candidate blocks, and selects the closest block.

### AssetOutput (Asset Output)
One of the three-layer outputs in EUTXO, representing transferable value such as tokens, NFTs, SFTs.

---

## B

### Block
A container for transactions, containing block header and block body, linked by hashes to form a blockchain.

### Block Header
Metadata of a block, containing version, previous block hash, Merkle root, state root, timestamp, difficulty, Nonce, etc.

---

## C

### Chain
An ordered collection of blocks, forming an immutable historical record.

### Consensus
A mechanism for multiple nodes to reach agreement on blockchain state. WES uses PoW+XOR hybrid consensus.

### CU (Compute Units)
Unit used to measure computing power in WES. Contracts and AI models use unified CU measurement.

---

## E

### EUTXO (Extended UTXO)
WES's state model, extending traditional UTXO with a three-layer output architecture and reference-without-consumption mode.

---

## G

### Genesis Block
The first block of a blockchain, with no previous block.

### Gossip
Message propagation protocol in P2P networks, achieving network-wide broadcast through inter-node message propagation.

---

## H

### HostABI (Host ABI)
Interface provided by ISPC execution environment to contracts, including UTXO operations, resource operations, external calls, etc.

---

## I

### ISPC (Intrinsic Self-Proving Computing)
WES's core computing paradigm, automatically generating verifiable zero-knowledge proofs during computation execution.

---

## K

### K-bucket
Routing table structure in Kademlia DHT, organizing nodes by XOR distance.

### Kademlia
Distributed hash table protocol based on XOR distance, used for node discovery and routing in P2P networks.

---

## M

### Mempool (Memory Pool/Transaction Pool)
Memory area storing pending transactions.

### Merkle Root
Root hash of a Merkle tree, used for efficient data integrity verification.

### Miner
Node role that performs PoW computation and produces candidate blocks.

---

## N

### Node
Computer participating in the WES network, can be a full node, miner node, or light node.

### Nonce
Value continuously adjusted in PoW computation, used to find block hash satisfying difficulty requirements.

---

## O

### ONNX (Open Neural Network Exchange)
Open Neural Network Exchange format. WES supports executing AI models in ONNX format.

### OutPoint
Unique identifier of UTXO, composed of transaction hash and output index.

---

## P

### P2P (Peer-to-Peer)
Decentralized network architecture where nodes connect directly to each other without a central server.

### PoW (Proof of Work)
Consensus mechanism that proves work through computational puzzles.

### Proof
In ISPC, refers to zero-knowledge proof used to verify computation correctness.

---

## R

### Reference Input
Input type in EUTXO that references but does not consume UTXO, used for reading shared data.

### Reorg (Reorganization)
Process of rolling back old chain and switching to new chain when a better fork is discovered.

### ResourceOutput (Resource Output)
One of the three-layer outputs in EUTXO, representing referable resources such as contracts, models, files.

---

## S

### StateOutput (State Output)
One of the three-layer outputs in EUTXO, representing execution results or state snapshots.

### StateRoot (State Root)
Merkle root of current state, used to verify state integrity.

---

## T

### Tip
The latest block of the current main chain.

### Transaction
Basic unit of state change, containing inputs, outputs, and execution information.

### TxID (Transaction Identifier)
Unique hash identifier of a transaction.

---

## U

### URES (Unified Resource Management)
WES's resource management system, unified management of various resources based on content addressing.

### UTXO (Unspent Transaction Output)
Transaction output that has not been consumed, representing available state units.

### State Unit (Cell)
Abstract concept at the protocol layer, representing "state unit". In the EUTXO model, includes:
- **AssetCell**: Asset layer state unit, carrying value units
- **ResourceCell**: Resource layer state unit, carrying or referencing resource objects
- **StateRecordCell**: State record layer state unit, recording execution results, audit information, and evidence

**Cell vs Output**:
- `Cell` is an abstract concept at the protocol layer, emphasizing the lifecycle of state units (creation, consumption, reference)
- `Output` is a concrete implementation at the transaction layer, emphasizing the output structure of transactions
- They correspond semantically: `AssetCell` corresponds to `AssetOutput`, `ResourceCell` corresponds to `ResourceOutput`, `StateRecordCell` corresponds to `StateOutput`

### Right Bearer
From the protocol perspective, an abstract "control carrier" that has specific rights to certain resources or states. Manifested through verifiable on-chain conditions (addresses, scripts, signature conditions), not equivalent to real-world individuals/organizations.

**Right Types**:
- **Ownership**: Has final disposal right over resource objects
- **Usage Right**: Right to use resources under given conditions
- **Management Right**: Right to adjust usage conditions, allocate usage rights, etc.

---

## W

### WASM (WebAssembly)
A portable binary instruction format. WES supports executing smart contracts in WASM format.

### WES (Weisyn)
Third-generation blockchain that defines the verifiable computing paradigm, supporting AI and enterprise applications running on-chain.

---

## X

### XOR Distance
XOR value of two identifiers, used in Kademlia routing and block selection in WES consensus.

---

## Z

### ZK Proof (Zero-Knowledge Proof)
Cryptographic technique that allows a prover to prove to a verifier that a statement is true without revealing any additional information.

