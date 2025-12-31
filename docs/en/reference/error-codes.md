# Error Code Reference

---

## Overview

This document defines error codes used in the WES system and their meanings.

---

## Error Code Format

Error code format: `WES-XXXX`

- `WES`: Error domain prefix
- `XXXX`: 4-digit error code

---

## General Errors (0xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-0001 | InternalError | Internal error |
| WES-0002 | InvalidParameter | Invalid parameter |
| WES-0003 | NotFound | Resource not found |
| WES-0004 | AlreadyExists | Resource already exists |
| WES-0005 | PermissionDenied | Insufficient permissions |
| WES-0006 | Timeout | Operation timeout |
| WES-0007 | Unavailable | Service unavailable |

---

## Transaction Errors (1xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-1001 | InvalidTxFormat | Invalid transaction format |
| WES-1002 | InvalidSignature | Invalid signature |
| WES-1003 | InsufficientBalance | Insufficient balance |
| WES-1004 | DuplicateTx | Duplicate transaction |
| WES-1005 | TxNotFound | Transaction not found |
| WES-1006 | TxExpired | Transaction expired |
| WES-1007 | TxRejected | Transaction rejected |
| WES-1008 | InvalidNonce | Invalid Nonce |

---

## UTXO Errors (2xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-2001 | UTXONotFound | UTXO not found |
| WES-2002 | UTXOAlreadySpent | UTXO already spent |
| WES-2003 | InvalidOutPoint | Invalid OutPoint |
| WES-2004 | InvalidUnlockProof | Invalid unlock proof |
| WES-2005 | ValueMismatch | Value not conserved |

---

## Execution Errors (3xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-3001 | ExecutionFailed | Execution failed |
| WES-3002 | OutOfGas | Computing resources exhausted |
| WES-3003 | InvalidContract | Invalid contract |
| WES-3004 | MethodNotFound | Method not found |
| WES-3005 | InvalidProof | Invalid proof |
| WES-3006 | ProofGenerationFailed | Proof generation failed |

---

## Resource Errors (4xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-4001 | ResourceNotFound | Resource not found |
| WES-4002 | InvalidResourceFormat | Invalid resource format |
| WES-4003 | ResourceTooLarge | Resource too large |
| WES-4004 | ResourceHashMismatch | Resource hash mismatch |

---

## Network Errors (5xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-5001 | ConnectionFailed | Connection failed |
| WES-5002 | PeerNotFound | Peer not found |
| WES-5003 | MessageTooLarge | Message too large |
| WES-5004 | ProtocolError | Protocol error |

---

## Consensus Errors (6xxx)

| Error Code | Name | Description |
|------------|------|-------------|
| WES-6001 | InvalidBlock | Invalid block |
| WES-6002 | InvalidPoW | Invalid PoW |
| WES-6003 | ForkDetected | Fork detected |
| WES-6004 | ReorgFailed | Reorganization failed |

---

## Error Handling Examples

### Go

```go
if err != nil {
    if wesErr, ok := err.(*wes.Error); ok {
        switch wesErr.Code {
        case "WES-1003":
            // Handle insufficient balance
        case "WES-3001":
            // Handle execution failure
        default:
            // Handle other errors
        }
    }
}
```

### JavaScript

```javascript
try {
    await client.sendTransaction(tx);
} catch (err) {
    if (err.code === 'WES-1003') {
        console.log('Insufficient balance');
    } else {
        console.log('Error:', err.message);
    }
}
```

---

## Related Documentation

- [API Reference](./api/) - API error responses
- [Troubleshooting](../how-to/troubleshoot/) - Problem diagnosis

