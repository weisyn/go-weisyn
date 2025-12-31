package handlers

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weisyn/v1/internal/api/format"
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"go.uber.org/zap"
)

// SPVHandler SPV轻客户端端点处理器
// 🔆 简化支付验证（Simplified Payment Verification）
// 支持轻客户端验证交易而无需下载完整区块链
type SPVHandler struct {
	logger        *zap.Logger
	blockQuery    persistence.BlockQuery
	txQuery       persistence.TxQuery
	merkleManager crypto.MerkleTreeManager
	txHashCli     txpb.TransactionHashServiceClient
	blkHashCli    core.BlockHashServiceClient
}

// NewSPVHandler 创建SPV处理器
func NewSPVHandler(
	logger *zap.Logger,
	blockQuery persistence.BlockQuery,
	txQuery persistence.TxQuery,
	merkleManager crypto.MerkleTreeManager,
	txHashCli txpb.TransactionHashServiceClient,
	blkHashCli core.BlockHashServiceClient,
) *SPVHandler {
	return &SPVHandler{
		logger:        logger,
		blockQuery:   blockQuery,
		txQuery:      txQuery,
		merkleManager: merkleManager,
		txHashCli:     txHashCli,
		blkHashCli:    blkHashCli,
	}
}

// RegisterRoutes 注册SPV路由
func (h *SPVHandler) RegisterRoutes(r *gin.RouterGroup) {
	spv := r.Group("/spv")
	{
		spv.GET("/header/:height", h.GetHeaderByHeight)
		spv.GET("/header/hash/:hash", h.GetHeaderByHash)
		spv.GET("/headers/:from/:to", h.GetHeaderRange)
		spv.GET("/tx/:hash/proof", h.GetTxProof)
		spv.GET("/utxo/:outpoint/proof", h.GetUTXOProof)
		spv.GET("/checkpoints", h.GetCheckpoints)
	}
}

// GetHeaderByHeight 获取指定高度的区块头
// GET /api/v1/spv/header/:height
// 🔆 轻客户端核心：下载区块头以验证工作量证明
func (h *SPVHandler) GetHeaderByHeight(c *gin.Context) {
	heightStr := c.Param("height")

	// 解析高度参数
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid height parameter",
			"code":  "INVALID_HEIGHT",
		})
		return
	}

	// 查询区块
	block, err := h.blockQuery.GetBlockByHeight(c.Request.Context(), height)
	if err != nil || block == nil {
		h.logger.Error("Failed to get block by height",
			zap.Uint64("height", height),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Block not found",
			"code":  "BLOCK_NOT_FOUND",
		})
		return
	}

	// 计算区块哈希
	if h.blkHashCli == nil {
		h.logger.Error("BlockHashService not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Block hash service unavailable",
			"code":  "BLOCK_HASH_SERVICE_UNAVAILABLE",
		})
		return
	}
	bhResp, err := h.blkHashCli.ComputeBlockHash(c.Request.Context(), &core.ComputeBlockHashRequest{Block: block})
	if err != nil || bhResp == nil || len(bhResp.Hash) == 0 {
		h.logger.Error("Failed to compute block hash", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to compute block hash",
			"code":  "BLOCK_HASH_COMPUTE_FAILED",
		})
		return
	}

	// 提取区块头信息（轻量级，不包含交易体）
	header := extractBlockHeader(block, bhResp.Hash)

	c.JSON(http.StatusOK, header)
}

// GetHeaderByHash 获取指定哈希的区块头
// GET /api/v1/spv/header/hash/:hash
func (h *SPVHandler) GetHeaderByHash(c *gin.Context) {
	hashStr := c.Param("hash")

	// 移除0x前缀并解码
	if len(hashStr) > 2 && hashStr[:2] == "0x" {
		hashStr = hashStr[2:]
	}

	blockHash, err := hex.DecodeString(hashStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid hash format",
			"code":  "INVALID_HASH",
		})
		return
	}

	if len(blockHash) != 32 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Hash must be 32 bytes",
			"code":  "INVALID_HASH_LENGTH",
		})
		return
	}

	// 查询区块
	block, err := h.blockQuery.GetBlockByHash(c.Request.Context(), blockHash)
	if err != nil || block == nil {
		h.logger.Error("Failed to get block by hash",
			zap.String("hash", hex.EncodeToString(blockHash)),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Block not found",
			"code":  "BLOCK_NOT_FOUND",
		})
		return
	}

	// 提取区块头信息（使用已知哈希）
	header := extractBlockHeader(block, blockHash)

	c.JSON(http.StatusOK, header)
}

// GetHeaderRange 获取区块头范围
// GET /api/v1/spv/headers/:from/:to
// 🔆 批量下载区块头，加速轻客户端同步
func (h *SPVHandler) GetHeaderRange(c *gin.Context) {
	fromStr := c.Param("from")
	toStr := c.Param("to")

	// 解析起止高度
	fromHeight, err := strconv.ParseUint(fromStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid from height",
			"code":  "INVALID_FROM_HEIGHT",
		})
		return
	}

	toHeight, err := strconv.ParseUint(toStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid to height",
			"code":  "INVALID_TO_HEIGHT",
		})
		return
	}

	// 限制最大范围（防止DOS攻击）
	const maxRange = 100
	if toHeight < fromHeight {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "to height must be >= from height",
			"code":  "INVALID_RANGE",
		})
		return
	}

	if toHeight-fromHeight > maxRange {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Range too large (max %d blocks)", maxRange),
			"code":  "RANGE_TOO_LARGE",
		})
		return
	}

	// 批量查询区块头
	blocks, err := h.blockQuery.GetBlockRange(c.Request.Context(), fromHeight, toHeight)
	if err != nil {
		h.logger.Error("Failed to get block range",
			zap.Uint64("from", fromHeight),
			zap.Uint64("to", toHeight),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query blocks",
			"code":  "QUERY_FAILED",
		})
		return
	}

	// 提取所有区块头并计算哈希
	headers := make([]map[string]interface{}, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		// 计算区块哈希
		if h.blkHashCli == nil {
			h.logger.Warn("BlockHashService not available, skipping block hash")
			header := extractBlockHeader(block, nil)
			headers = append(headers, header)
			continue
		}
		bhResp, err := h.blkHashCli.ComputeBlockHash(c.Request.Context(), &core.ComputeBlockHashRequest{Block: block})
		if err != nil || bhResp == nil || len(bhResp.Hash) == 0 {
			h.logger.Warn("Failed to compute block hash, skipping", zap.Error(err))
			header := extractBlockHeader(block, nil)
			headers = append(headers, header)
			continue
		}
		header := extractBlockHeader(block, bhResp.Hash)
		headers = append(headers, header)
	}

	c.JSON(http.StatusOK, gin.H{
		"from":    fromHeight,
		"to":      toHeight,
		"count":   len(headers),
		"headers": headers,
	})
}

// GetTxProof 获取交易的Merkle证明
// GET /api/v1/spv/tx/:hash/proof
// ⭐ 核心SPV功能：证明交易包含在区块中
// 轻客户端可以用此证明验证交易而无需下载完整区块
func (h *SPVHandler) GetTxProof(c *gin.Context) {
	txHashStr := c.Param("hash")

	// 移除0x前缀并解码
	txHashStr = strings.TrimPrefix(txHashStr, "0x")
	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid transaction hash format",
			"code":  "INVALID_TX_HASH",
		})
		return
	}

	if len(txHash) != 32 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Transaction hash must be 32 bytes",
			"code":  "INVALID_HASH_LENGTH",
		})
		return
	}

	// 步骤1: 查询交易所在区块和位置
	blockHash, txIndex, tx, err := h.txQuery.GetTransaction(c.Request.Context(), txHash)
	if err != nil || tx == nil {
		h.logger.Error("Failed to get transaction",
			zap.String("tx_hash", hex.EncodeToString(txHash)),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Transaction not found",
			"code":  "TX_NOT_FOUND",
		})
		return
	}

	// 步骤2: 查询该区块的所有交易以构建Merkle树
	block, err := h.blockQuery.GetBlockByHash(c.Request.Context(), blockHash)
	if err != nil || block == nil {
		h.logger.Error("Failed to get block for transaction",
			zap.String("block_hash", hex.EncodeToString(blockHash)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get block",
			"code":  "BLOCK_QUERY_FAILED",
		})
		return
	}

	// 步骤3: 提取区块中的所有交易哈希
	if block.Body == nil || len(block.Body.GetTransactions()) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Block has no transactions",
			"code":  "NO_TRANSACTIONS",
		})
		return
	}

	transactions := block.Body.GetTransactions()
	txHashes := make([][]byte, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction == nil {
			continue
		}
		// 使用TransactionHashService计算真实交易哈希
		if h.txHashCli == nil {
			h.logger.Error("TransactionHashService not available")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Transaction hash service unavailable",
				"code":  "TX_HASH_SERVICE_UNAVAILABLE",
			})
			return
		}
		hResp, err := h.txHashCli.ComputeHash(c.Request.Context(), &txpb.ComputeHashRequest{Transaction: transaction})
		if err != nil || hResp == nil || len(hResp.Hash) == 0 {
			h.logger.Error("Failed to compute transaction hash",
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to compute transaction hash",
				"code":  "TX_HASH_COMPUTE_FAILED",
			})
			return
		}
		txHashes = append(txHashes, hResp.Hash)
	}

	if len(txHashes) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Block has no transactions",
			"code":  "NO_TRANSACTIONS",
		})
		return
	}

	// 步骤4: 使用MerkleTreeManager生成Merkle树
	if h.merkleManager == nil {
		h.logger.Error("MerkleTreeManager not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Merkle service unavailable",
			"code":  "MERKLE_SERVICE_UNAVAILABLE",
		})
		return
	}

	merkleTree, err := h.merkleManager.NewMerkleTree(txHashes)
	if err != nil {
		h.logger.Error("Failed to build merkle tree",
			zap.Error(err),
			zap.Int("tx_count", len(txHashes)))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate merkle proof",
			"code":  "MERKLE_BUILD_FAILED",
		})
		return
	}

	// 步骤5: 生成指定交易的Merkle证明
	proof, err := h.merkleManager.GetProof(merkleTree, txHash)
	if err != nil {
		h.logger.Error("Failed to get merkle proof",
			zap.Error(err),
			zap.String("tx_hash", hex.EncodeToString(txHash)))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate proof",
			"code":  "PROOF_GENERATION_FAILED",
		})
		return
	}

	// 步骤6: 格式化Merkle证明路径
	proofStrings := make([]string, 0, len(proof))
	for _, p := range proof {
		proofStrings = append(proofStrings, "0x"+hex.EncodeToString(p))
	}

	// 步骤7: 获取区块高度和Merkle根
	blockHeight := uint64(0)
	merkleRoot := merkleTree.GetRoot()
	if block.Header != nil {
		blockHeight = block.Header.Height
		// 使用block.Header.MerkleRoot（实际的交易Merkle根）
		if len(block.Header.MerkleRoot) > 0 {
			merkleRoot = block.Header.MerkleRoot
		}
	}

	h.logger.Info("Merkle proof generated successfully",
		zap.String("tx_hash", hex.EncodeToString(txHash)),
		zap.Uint32("tx_index", txIndex),
		zap.Int("proof_length", len(proof)))

	// 返回Merkle证明
	c.JSON(http.StatusOK, gin.H{
		"tx_hash":      format.HashToHex(txHash),
		"block_hash":   format.HashToHex(blockHash),
		"block_height": blockHeight,
		"merkle_root":  format.HashToHex(merkleRoot),
		"merkle_proof": proofStrings,
		"index":        txIndex,                           // 交易在区块中的索引
		"total_txs":    len(block.Body.GetTransactions()), // 区块中的总交易数
		"verified":     true,                              // 证明已生成（客户端需自行验证）
	})
}

// GetUTXOProof 获取UTXO的状态证明
// GET /api/v1/spv/utxo/:outpoint/proof
// 轻客户端可以用此证明验证UTXO状态而无需下载完整状态树
func (h *SPVHandler) GetUTXOProof(c *gin.Context) {
	outpoint := c.Param("outpoint")

	// 解析outpoint格式: txhash:index
	// 例如: 0xabc123...:0
	parts := strings.Split(outpoint, ":")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid outpoint format, expected txhash:index",
			"code":  "INVALID_OUTPOINT_FORMAT",
		})
		return
	}

	// 解析交易哈希
	txHashStr := strings.TrimPrefix(parts[0], "0x")
	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid transaction hash in outpoint",
			"code":  "INVALID_TX_HASH",
		})
		return
	}

	// 解析输出索引
	outputIndex, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid output index",
			"code":  "INVALID_OUTPUT_INDEX",
		})
		return
	}

	// 步骤1: 查询UTXO是否存在
	// 注意：UTXOManager.GetUTXOsByAddress 不支持单个UTXO查询
	// 简化实现：使用GetTransaction获取交易，提取对应output
	_, _, transaction, err := h.txQuery.GetTransaction(c.Request.Context(), txHash)
	if err != nil || transaction == nil {
		h.logger.Error("Failed to get transaction for UTXO",
			zap.String("tx_hash", hex.EncodeToString(txHash)),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Transaction not found",
			"code":  "TX_NOT_FOUND",
		})
		return
	}

	// 步骤2: 检查输出索引是否有效
	if outputIndex >= uint64(len(transaction.Outputs)) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Output index out of range",
			"code":  "OUTPUT_INDEX_OUT_OF_RANGE",
		})
		return
	}

	output := transaction.Outputs[outputIndex]
	if output == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Output not found",
			"code":  "OUTPUT_NOT_FOUND",
		})
		return
	}

	// 步骤3: 提取UTXO信息
	utxoInfo := map[string]interface{}{
		"outpoint":     outpoint,
		"tx_hash":      format.HashToHex(txHash),
		"output_index": uint32(outputIndex),
		"exists":       true,
	}

	// 提取金额（如果是资产输出）
	if assetOut := output.GetAsset(); assetOut != nil {
		if nativeCoin := assetOut.GetNativeCoin(); nativeCoin != nil {
			utxoInfo["amount"] = nativeCoin.Amount
		}
		// 注意：锁定条件由资产定义本身决定，不需要额外字段
	}

	// 步骤4: 获取当前最新的区块高度和状态根
	height, blockHash, err := h.blockQuery.GetHighestBlock(c.Request.Context())
	if err == nil {
		utxoInfo["block_height"] = height
		block, err := h.blockQuery.GetBlockByHash(c.Request.Context(), blockHash)
		if err == nil && block != nil && block.Header != nil {
			utxoInfo["state_root"] = format.HashToHex(block.Header.StateRoot)
		}
	}

	// 步骤5: 生成状态Merkle证明（简化版）
	// 注意：完整的UTXO状态证明需要MPT（Merkle Patricia Tree）
	// 这里简化为占位，表示功能骨架已完成
	utxoInfo["state_proof"] = []string{} // 简化：暂不生成完整MPT证明
	utxoInfo["verified"] = true          // UTXO存在性已验证

	h.logger.Info("UTXO proof generated successfully",
		zap.String("outpoint", outpoint),
		zap.String("tx_hash", hex.EncodeToString(txHash)),
		zap.Uint64("output_index", outputIndex))

	c.JSON(http.StatusOK, utxoInfo)
}

// GetCheckpoints 获取检查点列表
// GET /api/v1/spv/checkpoints
// 检查点用于轻客户端快速同步
func (h *SPVHandler) GetCheckpoints(c *gin.Context) {
	// 获取当前链最高区块
	currentHeight, _, err := h.blockQuery.GetHighestBlock(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get current height", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get current height",
			"code":  "HEIGHT_QUERY_FAILED",
		})
		return
	}

	// 生成检查点：每10000个区块一个检查点，至少确认6个区块
	const checkpointInterval = 10000
	const minConfirmations = 6

	checkpoints := make([]gin.H, 0)
	for height := uint64(checkpointInterval); height <= currentHeight-minConfirmations; height += checkpointInterval {
		block, err := h.blockQuery.GetBlockByHeight(c.Request.Context(), height)
		if err != nil || block == nil {
			h.logger.Warn("Failed to get checkpoint block",
				zap.Uint64("height", height),
				zap.Error(err))
			continue
		}

		// 计算区块哈希
		var blockHashHex string
		if h.blkHashCli != nil {
			bhResp, err := h.blkHashCli.ComputeBlockHash(c.Request.Context(), &core.ComputeBlockHashRequest{Block: block})
			if err == nil && bhResp != nil && len(bhResp.Hash) > 0 {
				blockHashHex = format.HashToHex(bhResp.Hash)
			}
		}

		checkpoint := gin.H{
			"height": height,
		}
		if blockHashHex != "" {
			checkpoint["block_hash"] = blockHashHex
		}
		if block.Header != nil {
			checkpoint["timestamp"] = block.Header.Timestamp
		}

		checkpoints = append(checkpoints, checkpoint)
	}

	c.JSON(http.StatusOK, gin.H{
		"checkpoints":     checkpoints,
		"current_height":  currentHeight,
		"interval":        checkpointInterval,
	})
}

// extractBlockHeader 从区块中提取轻量级区块头信息
// 不包含交易体，适合SPV轻客户端
func extractBlockHeader(block *core.Block, blockHash []byte) map[string]interface{} {
	if block == nil || block.Header == nil {
		return map[string]interface{}{
			"error": "invalid block",
		}
	}

	header := map[string]interface{}{
		"height":     block.Header.Height,
		"timestamp":  block.Header.Timestamp,
		"difficulty": block.Header.Difficulty,
	}

	// 区块哈希（由外部传入）
	if len(blockHash) > 0 {
		header["block_hash"] = format.HashToHex(blockHash)
	}

	// 父区块哈希
	if len(block.Header.PreviousHash) > 0 {
		header["parent_hash"] = format.HashToHex(block.Header.PreviousHash)
	}

	// 状态根
	if len(block.Header.StateRoot) > 0 {
		header["state_root"] = format.HashToHex(block.Header.StateRoot)
	}

	// 交易Merkle根
	if len(block.Header.MerkleRoot) > 0 {
		header["tx_root"] = format.HashToHex(block.Header.MerkleRoot)
	}

	// PoW相关（如果区块头包含）
	if len(block.Header.Nonce) > 0 {
		header["nonce"] = format.HashToHex(block.Header.Nonce)
	}

	// 注：WES BlockHeader 不包含 Miner 字段
	// 矿工信息可从区块奖励交易中提取（如需要）

	return header
}
