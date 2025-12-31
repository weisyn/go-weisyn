package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: inspect_block <block_file_path> [tx_hash_to_find]")
		os.Exit(1)
	}

	blockFile := os.Args[1]
	targetTxHash := ""
	if len(os.Args) >= 3 {
		targetTxHash = os.Args[2]
	}

	// 读取区块文件
	blockData, err := ioutil.ReadFile(blockFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取区块文件失败: %v\n", err)
		os.Exit(1)
	}

	// 解析区块
	var block core.Block
	if err := proto.Unmarshal(blockData, &block); err != nil {
		fmt.Fprintf(os.Stderr, "解析区块失败: %v\n", err)
		os.Exit(1)
	}

	// 打印区块信息
	header := block.GetHeader()
	if header == nil {
		fmt.Fprintf(os.Stderr, "区块头缺失\n")
		os.Exit(1)
	}

	// 计算区块哈希
	headerBytes, err := proto.Marshal(header)
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化区块头失败: %v\n", err)
		os.Exit(1)
	}
	// 使用 Double SHA-256 计算区块哈希（与挖矿保持一致）
	firstHash := sha256.Sum256(headerBytes)
	blockHash := sha256.Sum256(firstHash[:])

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("区块信息\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("高度: %d\n", header.GetHeight())
	fmt.Printf("区块哈希: %x\n", blockHash)
	fmt.Printf("前一个区块哈希: %x\n", header.GetPreviousHash())
	fmt.Printf("Merkle根: %x\n", header.GetMerkleRoot())
	fmt.Printf("时间戳: %d\n", header.GetTimestamp())
	fmt.Printf("难度: %d\n", header.GetDifficulty())
	fmt.Printf("交易数量: %d\n", len(block.GetBody().GetTransactions()))
	fmt.Printf("\n")

	// 解析目标交易哈希
	var targetTxHashBytes []byte
	if targetTxHash != "" {
		var err error
		targetTxHashBytes, err = hex.DecodeString(targetTxHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无效的交易哈希: %v\n", err)
			os.Exit(1)
		}
	}

	// 遍历交易
	body := block.GetBody()
	if body == nil {
		fmt.Println("区块体为空")
		os.Exit(0)
	}

	txs := body.GetTransactions()
	for i, tx := range txs {
		// 计算交易哈希（排除签名字段）
		txCopy := proto.Clone(tx).(*transaction.Transaction)
		// 清空所有输入的解锁证明（包含签名）
		for _, input := range txCopy.Inputs {
			input.UnlockingProof = nil
		}
		// 序列化交易（已排除签名）进行哈希计算
		mo := proto.MarshalOptions{Deterministic: true}
		txBytes, err := mo.Marshal(txCopy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "序列化交易失败: %v\n", err)
			continue
		}
		txHash := sha256.Sum256(txBytes)

		// 如果指定了目标交易哈希，只检查匹配的交易
		if len(targetTxHashBytes) > 0 {
			if len(txHash) != len(targetTxHashBytes) {
				continue
			}
			match := true
			for j := 0; j < len(txHash); j++ {
				if txHash[j] != targetTxHashBytes[j] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("交易 [%d]\n", i)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("交易哈希: %x\n", txHash)
		fmt.Printf("版本: %d\n", tx.GetVersion())
		fmt.Printf("\n")

		// 检查输入
		inputs := tx.GetInputs()
		fmt.Printf("输入数量: %d\n", len(inputs))
		for j, input := range inputs {
			fmt.Printf("\n  输入 [%d]:\n", j)
			prevOutput := input.GetPreviousOutput()
			if prevOutput != nil {
				fmt.Printf("    引用交易哈希: %x\n", prevOutput.GetTxId())
				fmt.Printf("    输出索引: %d\n", prevOutput.GetOutputIndex())
			}
			fmt.Printf("    引用模式: %v (true=只读引用不消费, false=消费引用)\n", input.GetIsReferenceOnly())

			// 检查解锁证明
			switch proof := input.GetUnlockingProof().(type) {
			case *transaction.TxInput_ExecutionProof:
				fmt.Printf("    解锁证明类型: ExecutionProof (ISPC执行证明)\n")
				execProof := proof.ExecutionProof
				if execProof != nil {
					fmt.Printf("      执行结果哈希: %x\n", execProof.GetExecutionResultHash())
					fmt.Printf("      状态转换证明: %d 字节\n", len(execProof.GetStateTransitionProof()))
					fmt.Printf("      执行时间: %d ms\n", execProof.GetExecutionTimeMs())
					if ctx := execProof.GetContext(); ctx != nil {
						fmt.Printf("      执行上下文:\n")
						fmt.Printf("        资源地址: %x\n", ctx.GetResourceAddress())
						fmt.Printf("        执行类型: %v\n", ctx.GetExecutionType())
						fmt.Printf("        输入数据哈希: %x\n", ctx.GetInputDataHash())
						fmt.Printf("        输出数据哈希: %x\n", ctx.GetOutputDataHash())
					}
				}
			case *transaction.TxInput_SingleKeyProof:
				fmt.Printf("    解锁证明类型: SingleKeyProof\n")
			case *transaction.TxInput_MultiKeyProof:
				fmt.Printf("    解锁证明类型: MultiKeyProof\n")
			default:
				fmt.Printf("    解锁证明类型: 其他\n")
			}
		}
		fmt.Printf("\n")

		// 检查输出
		outputs := tx.GetOutputs()
		fmt.Printf("输出数量: %d\n", len(outputs))
		for j, output := range outputs {
			fmt.Printf("\n  输出 [%d]:\n", j)
			fmt.Printf("    所有者: %x\n", output.GetOwner())

			switch out := output.GetOutputContent().(type) {
			case *transaction.TxOutput_Resource:
				fmt.Printf("    类型: Resource (资源输出)\n")
				resource := out.Resource
				if resource != nil && resource.GetResource() != nil {
					contentHash := resource.GetResource().GetContentHash()
					fmt.Printf("    内容哈希: %x\n", contentHash)
					fmt.Printf("    名称: %s\n", resource.GetResource().GetName())
					fmt.Printf("    可执行类型: %s\n", resource.GetResource().GetExecutableType().String())
				}
			case *transaction.TxOutput_Asset:
				fmt.Printf("    类型: Asset (资产输出)\n")
				asset := out.Asset
				if asset != nil {
					assetContent := asset.GetAssetContent()
					switch ac := assetContent.(type) {
					case *transaction.AssetOutput_NativeCoin:
						if nativeCoin := ac.NativeCoin; nativeCoin != nil {
							fmt.Printf("    原生币金额: %s\n", nativeCoin.GetAmount())
						}
					case *transaction.AssetOutput_ContractToken:
						if contractToken := ac.ContractToken; contractToken != nil {
							fmt.Printf("    合约代币金额: %s\n", contractToken.GetAmount())
							fmt.Printf("    合约地址: %x\n", contractToken.GetContractAddress())
						}
					default:
						fmt.Printf("    资产内容类型: 其他\n")
					}
				}
			case *transaction.TxOutput_State:
				fmt.Printf("    类型: State (状态输出)\n")
				state := out.State
				if state != nil {
					fmt.Printf("    状态ID: %x\n", state.GetStateId())
					fmt.Printf("    状态版本: %d\n", state.GetStateVersion())
					if zkProof := state.GetZkProof(); zkProof != nil {
						fmt.Printf("    ✅ ZK证明存在\n")
						if vkHash := zkProof.GetVerificationKeyHash(); len(vkHash) > 0 {
							fmt.Printf("    验证密钥哈希: %x\n", vkHash)
						}
					} else {
						fmt.Printf("    ⚠️  ZK证明为空\n")
					}
				}
			default:
				fmt.Printf("    类型: 其他\n")
			}
		}
		fmt.Printf("\n")

		// 如果指定了目标交易，只检查这一个
		if len(targetTxHashBytes) > 0 {
			break
		}
	}
}
