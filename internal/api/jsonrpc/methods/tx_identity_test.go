package methods

import (
	"testing"

	ecdsacrypto "github.com/ethereum/go-ethereum/crypto"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

func TestPopulateExecutionProofIdentities(t *testing.T) {
	methods := &TxMethods{}

	privateKey, err := ecdsacrypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey := ecdsacrypto.CompressPubkey(&privateKey.PublicKey)

	tx := &txpb.Transaction{
		Inputs: []*txpb.TxInput{
			{
				UnlockingProof: &txpb.TxInput_ExecutionProof{
					ExecutionProof: &txpb.ExecutionProof{
						Context: &txpb.ExecutionProof_ExecutionContext{
							CallerIdentity: &txpb.IdentityProof{
								ContextHash: make([]byte, 32),
							},
						},
					},
				},
			},
		},
	}

	baseNonce := make([]byte, 32)
	baseNonce[31] = 1

	if err := methods.populateExecutionProofIdentities(tx, privateKey, publicKey, baseNonce); err != nil {
		t.Fatalf("populate identities: %v", err)
	}

	proof := tx.Inputs[0].GetExecutionProof()
	if len(proof.Context.CallerIdentity.Signature) != 64 {
		t.Fatalf("signature length should be 64 bytes")
	}
	if len(proof.Context.CallerIdentity.PublicKey) != len(publicKey) {
		t.Fatalf("public key should be stored in identity proof")
	}
	if len(proof.Context.CallerIdentity.Nonce) != 32 {
		t.Fatalf("nonce should be 32 bytes")
	}
	if proof.Context.CallerIdentity.Timestamp == 0 {
		t.Fatalf("timestamp should be populated")
	}
}
