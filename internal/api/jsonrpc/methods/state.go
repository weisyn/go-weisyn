package methods

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/weisyn/v1/internal/api/format"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	cryptoInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/utils"
	"go.uber.org/zap"
)

// StateMethods 状态查询相关方法
type StateMethods struct {
	logger          *zap.Logger
	accountQuery    persistence.AccountQuery
	utxoQuery       persistence.UTXOQuery
	blockQuery      persistence.BlockQuery
	ispcCoordinator ispc.ISPCCoordinator           // 使用 ISPC Coordinator 代替直接的 WASM Engine
	addressManager  cryptoInterface.AddressManager // 地址管理器，用于验证Base58格式地址
}

// NewStateMethods 创建状态方法处理器
func NewStateMethods(
	logger *zap.Logger,
	accountQuery persistence.AccountQuery,
	utxoQuery persistence.UTXOQuery,
	blockQuery persistence.BlockQuery,
	ispcCoordinator ispc.ISPCCoordinator,
	addressManager cryptoInterface.AddressManager,
) *StateMethods {
	return &StateMethods{
		logger:          logger,
		accountQuery:    accountQuery,
		utxoQuery:       utxoQuery,
		blockQuery:      blockQuery,
		ispcCoordinator: ispcCoordinator,
		addressManager:  addressManager,
	}
}

// GetBalance 查询账户余额
// Method: wes_getBalance
// Params: [address: string, blockParam: object (optional)]
// address: Base58格式的WES地址（如CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR）
// blockParam 示例: {"blockHeight": "0x1234"} 或 {"blockHash": "0xabc..."}
func (m *StateMethods) GetBalance(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing address", nil)
	}

	// 1. 解析地址参数（WES使用Base58格式，不兼容ETH的0x前缀格式）
	addressStr, ok := args[0].(string)
	if !ok {
		return nil, NewInvalidParamsError("address must be string", nil)
	}

	// 验证并转换Base58格式地址
	if m.addressManager == nil {
		return nil, NewInternalError("address manager not available", nil)
	}

	// 拒绝0x前缀的ETH地址格式
	if len(addressStr) > 2 && (addressStr[:2] == "0x" || addressStr[:2] == "0X") {
		return nil, NewInvalidParamsError("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式", nil)
	}

	// 验证Base58格式地址
	validAddress, err := m.addressManager.StringToAddress(addressStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid address format: %v", err), nil)
	}

	// 转换为字节数组
	address, err := m.addressManager.AddressToBytes(validAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("failed to convert address: %v", err), nil)
	}

	// 2. 解析状态锚点参数（atHeight/atHash）- 可选
	var anchorHeight uint64
	var anchorHash []byte
	if len(args) > 1 {
		if blockParam, ok := args[1].(map[string]interface{}); ok {
			if heightStr, ok := blockParam["blockHeight"].(string); ok {
				if len(heightStr) > 2 && heightStr[:2] == "0x" {
					heightStr = heightStr[2:]
				}
				_, err := fmt.Sscanf(heightStr, "%x", &anchorHeight)
				if err != nil {
					return nil, NewInvalidParamsError(fmt.Sprintf("invalid blockHeight: %v", err), nil)
				}
			}
			if hashStr, ok := blockParam["blockHash"].(string); ok {
				if len(hashStr) > 2 && hashStr[:2] == "0x" {
					hashStr = hashStr[2:]
				}
				anchorHash, err = hex.DecodeString(hashStr)
				if err != nil {
					return nil, NewInvalidParamsError(fmt.Sprintf("invalid blockHash: %v", err), nil)
				}
			}
		}
	}

	// 3. 调用accountQuery.GetAccountBalance()
	if m.accountQuery == nil {
		return nil, NewInternalError("account query not available", nil)
	}
	balanceInfo, err := m.accountQuery.GetAccountBalance(ctx, address, nil) // nil表示原生代币
	if err != nil {
		m.logger.Error("Failed to get balance",
			zap.String("address", hex.EncodeToString(address)),
			zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	// 4. 获取状态锚点信息（高度、哈希、状态根、时间戳）
	var height uint64
	var blockHash []byte
	var stateRoot []byte
	var timestamp int64

	if anchorHeight > 0 {
		// 使用指定高度的锚点
		height = anchorHeight
		block, err := m.blockQuery.GetBlockByHeight(ctx, height)
		if err == nil && block != nil && block.Header != nil {
			// 需要通过 BlockHashService 计算区块哈希
			stateRoot = block.Header.StateRoot
			timestamp = int64(block.Header.Timestamp)
		}
	} else if len(anchorHash) > 0 {
		// 使用指定哈希的锚点
		block, err := m.blockQuery.GetBlockByHash(ctx, anchorHash)
		if err == nil && block != nil && block.Header != nil {
			blockHash = anchorHash
			height = block.Header.Height
			stateRoot = block.Header.StateRoot
			timestamp = int64(block.Header.Timestamp)
		}
	} else {
		// 使用最新状态
		h, bHash, err := m.blockQuery.GetHighestBlock(ctx)
		if err == nil {
			height = h
			blockHash = bHash
			block, err := m.blockQuery.GetBlockByHash(ctx, blockHash)
			if err == nil && block != nil && block.Header != nil {
				stateRoot = block.Header.StateRoot
				timestamp = int64(block.Header.Timestamp)
			}
		}
	}

	// 🔍 DEBUG: 打印余额信息
	m.logger.Info("🔍 [DEBUG] GetBalance 返回",
		zap.String("address", hex.EncodeToString(address)),
		zap.Uint64("available_wei", balanceInfo.Available),
		zap.String("balance_hex", fmt.Sprintf("0x%x", balanceInfo.Available)),
	)

	// 构造响应（包含状态锚点）
	resp := map[string]interface{}{
		// balance: 最小单位（BaseUnit），用于程序计算（保持兼容）
		"balance": balanceInfo.Available,
		// balance_wes: 用户展示用（WES单位，8位小数）
		"balance_wes": utils.FormatWeiToDecimal(balanceInfo.Available),
		"decimals":    utils.Decimals,
		"wei_per_wes": utils.WeiPer,
		"height":      height,
	}
	if len(blockHash) > 0 {
		resp["block_hash"] = format.HashToHex(blockHash)
	}
	if len(stateRoot) > 0 {
		resp["state_root"] = format.HashToHex(stateRoot)
	}
	if timestamp > 0 {
		resp["timestamp"] = timestamp
	}

	return resp, nil
}

// GetContractTokenBalance 查询账户的合约代币余额
// Method: wes_getContractTokenBalance
// Params: [{ "address": "<Base58地址>", "content_hash": "<合约内容哈希>", "token_id": "<代币标识，可选>" }]
func (m *StateMethods) GetContractTokenBalance(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Address     string `json:"address"`
		ContentHash string `json:"content_hash"`
		TokenID     string `json:"token_id,omitempty"`
	}

	// JSON-RPC 可能以 [{...}] 或 {...} 形式传参
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		data, err := json.Marshal(paramsArray[0])
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("marshal params object failed: %v", err), nil)
		}
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params object: %v", err), nil)
		}
	} else {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
		}
	}

	req.Address = strings.TrimSpace(req.Address)
	req.ContentHash = strings.TrimSpace(strings.TrimPrefix(req.ContentHash, "0x"))
	req.TokenID = strings.TrimSpace(req.TokenID)

	if req.Address == "" {
		return nil, NewInvalidParamsError("address is required", nil)
	}
	if req.ContentHash == "" {
		return nil, NewInvalidParamsError("content_hash is required", nil)
	}
	if len(req.ContentHash) != 64 {
		return nil, NewInvalidParamsError("content_hash must be 32-byte hex string", nil)
	}

	// 地址解析
	if m.addressManager == nil {
		return nil, NewInternalError("address manager not available", nil)
	}
	validAddress, err := m.addressManager.StringToAddress(req.Address)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid address format: %v", err), nil)
	}
	addressBytes, err := m.addressManager.AddressToBytes(validAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("failed to convert address: %v", err), nil)
	}

	// 合约内容哈希解析
	contentHashBytes, err := hex.DecodeString(req.ContentHash)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid content_hash: %v", err), nil)
	}

	// 根据内容哈希推导合约地址（20字节 hash160）
	contractAddrBytes := hash160(contentHashBytes)

	// TokenID 默认使用 "default"
	tokenIDStr := req.TokenID
	if tokenIDStr == "" {
		tokenIDStr = "default"
	}
	tokenIDBytes := []byte(tokenIDStr)

	if m.utxoQuery == nil {
		return nil, NewInternalError("utxo query not available", nil)
	}

	category := utxopb.UTXOCategory_UTXO_CATEGORY_ASSET
	utxos, err := m.utxoQuery.GetUTXOsByAddress(ctx, addressBytes, &category, true)
	if err != nil {
		m.logger.Error("Failed to get UTXOs for contract balance",
			zap.String("address", hex.EncodeToString(addressBytes)),
			zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	total := big.NewInt(0)
	utxoCount := 0

	for _, utxoObj := range utxos {
		if utxoObj == nil {
			continue
		}
		output := utxoObj.GetCachedOutput()
		if output == nil {
			continue
		}
		asset := output.GetAsset()
		if asset == nil {
			continue
		}
		contractToken := asset.GetContractToken()
		if contractToken == nil {
			continue
		}

		if !bytes.Equal(contractToken.GetContractAddress(), contractAddrBytes) {
			continue
		}

		fungibleID := contractToken.GetFungibleClassId()
		if len(fungibleID) == 0 {
			continue
		}
		if !bytes.Equal(fungibleID, tokenIDBytes) {
			continue
		}

		amountStr := contractToken.Amount
		if amountStr == "" {
			continue
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			continue
		}

		total.Add(total, amount)
		utxoCount++
	}

	// 状态锚点信息（与 wes_getBalance 保持一致）
	var height uint64
	var blockHash []byte
	var stateRoot []byte
	var timestamp int64

	if m.blockQuery != nil {
		h, bHash, err := m.blockQuery.GetHighestBlock(ctx)
		if err == nil {
			height = h
			blockHash = bHash
			if block, err := m.blockQuery.GetBlockByHash(ctx, blockHash); err == nil && block != nil && block.Header != nil {
				stateRoot = block.Header.StateRoot
				timestamp = int64(block.Header.Timestamp)
			}
		}
	}

	// 构造返回
	// 将合约地址转换为 Base58Check 格式
	contractAddress := format.MustAddressToBase58(contractAddrBytes, m.addressManager)

	response := map[string]interface{}{
		"address":          req.Address,
		"content_hash":     strings.ToLower(req.ContentHash), // 不带 0x 前缀
		"contract_address": contractAddress,                  // Base58Check 格式
		"token_id":         tokenIDStr,
		"balance":          total.String(),
		"utxo_count":       utxoCount,
		"height":           height,
	}

	if total.IsUint64() {
		response["balance_uint64"] = total.Uint64()
	}
	if len(blockHash) > 0 {
		response["block_hash"] = format.HashToHex(blockHash)
	}
	if len(stateRoot) > 0 {
		response["state_root"] = format.HashToHex(stateRoot)
	}
	if timestamp > 0 {
		response["timestamp"] = timestamp
	}

	return response, nil
}

// GetUTXO 查询UTXO
// Method: wes_getUTXO
// Params: [address: string, blockParam: object (optional)]
// address: Base58格式的WES地址（如CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR）
func (m *StateMethods) GetUTXO(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing address", nil)
	}

	// 1. 解析地址参数（WES使用Base58格式，不兼容ETH的0x前缀格式）
	addressStr, ok := args[0].(string)
	if !ok {
		return nil, NewInvalidParamsError("address must be string", nil)
	}

	// 验证并转换Base58格式地址
	if m.addressManager == nil {
		return nil, NewInternalError("address manager not available", nil)
	}

	// 拒绝0x前缀的ETH地址格式
	if len(addressStr) > 2 && (addressStr[:2] == "0x" || addressStr[:2] == "0X") {
		return nil, NewInvalidParamsError("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式", nil)
	}

	// 验证Base58格式地址
	validAddress, err := m.addressManager.StringToAddress(addressStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid address format: %v", err), nil)
	}

	// 转换为字节数组
	address, err := m.addressManager.AddressToBytes(validAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("failed to convert address: %v", err), nil)
	}

	// 2. 解析状态锚点参数（可选）
	var anchorHeight uint64
	var anchorHash []byte
	if len(args) > 1 {
		if blockParam, ok := args[1].(map[string]interface{}); ok {
			if heightStr, ok := blockParam["blockHeight"].(string); ok {
				if len(heightStr) > 2 && heightStr[:2] == "0x" {
					heightStr = heightStr[2:]
				}
				_, err := fmt.Sscanf(heightStr, "%x", &anchorHeight)
				if err != nil {
					return nil, NewInvalidParamsError(fmt.Sprintf("invalid blockHeight: %v", err), nil)
				}
			}
			if hashStr, ok := blockParam["blockHash"].(string); ok {
				if len(hashStr) > 2 && hashStr[:2] == "0x" {
					hashStr = hashStr[2:]
				}
				anchorHash, err = hex.DecodeString(hashStr)
				if err != nil {
					return nil, NewInvalidParamsError(fmt.Sprintf("invalid blockHash: %v", err), nil)
				}
			}
		}
	}

	// 3. 调用utxoQuery.GetUTXOsByAddress()
	if m.utxoQuery == nil {
		return nil, NewInternalError("utxo query not available", nil)
	}
	utxos, err := m.utxoQuery.GetUTXOsByAddress(ctx, address, nil, true)
	if err != nil {
		m.logger.Error("Failed to get UTXOs",
			zap.String("address", hex.EncodeToString(address)),
			zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	// 4. 格式化UTXO列表
	utxoList := make([]interface{}, 0, len(utxos))
	for _, utxo := range utxos {
		if utxo == nil || utxo.Outpoint == nil {
			continue
		}
		utxoItem := map[string]interface{}{
			"outpoint": fmt.Sprintf("%s:%d", hex.EncodeToString(utxo.Outpoint.TxId), utxo.Outpoint.OutputIndex),
			"height":   fmt.Sprintf("0x%x", utxo.BlockHeight),
		}
		// 如果有缓存的output，可以获取amount和tokenID信息
		if cachedOutput := utxo.GetCachedOutput(); cachedOutput != nil {
			if assetOut := cachedOutput.GetAsset(); assetOut != nil {
				if nativeCoin := assetOut.GetNativeCoin(); nativeCoin != nil {
					// 原生币（amount: BaseUnit 字符串；amount_wes: 用户展示用）
					utxoItem["amount"] = nativeCoin.Amount
					if amt, err := utils.TryParseAmountUint64(nativeCoin.Amount); err == nil {
						utxoItem["amount_wes"] = utils.FormatWeiToDecimal(amt)
					}
					// 原生币没有 tokenID，不设置 tokenID 字段
				} else if contractToken := assetOut.GetContractToken(); contractToken != nil {
					// 合约代币
					utxoItem["amount"] = contractToken.Amount
					// 提取 tokenID（从 TokenIdentifier oneof 中）
					// 注意：GetTokenIdentifier() 返回接口类型，需要使用类型断言
					if fungibleID := contractToken.GetFungibleClassId(); len(fungibleID) > 0 {
						utxoItem["tokenID"] = hex.EncodeToString(fungibleID)
					} else if nftID := contractToken.GetNftUniqueId(); len(nftID) > 0 {
						utxoItem["tokenID"] = hex.EncodeToString(nftID)
					} else if semiFungibleID := contractToken.GetSemiFungibleId(); semiFungibleID != nil {
						// SemiFungibleId 是结构体，需要序列化或提取关键字段
						// 简化：使用结构体的字符串表示（实际应该提取具体字段）
						utxoItem["tokenID"] = hex.EncodeToString([]byte(semiFungibleID.String()))
					}
					// 合约地址
					if len(contractToken.ContractAddress) > 0 {
						utxoItem["contractAddress"] = hex.EncodeToString(contractToken.ContractAddress)
					}
				}
			}
		}
		utxoList = append(utxoList, utxoItem)
	}

	// 5. 获取状态锚点信息
	var height uint64
	var blockHash []byte
	var stateRoot []byte
	var timestamp int64

	if anchorHeight > 0 {
		height = anchorHeight
		block, err := m.blockQuery.GetBlockByHeight(ctx, height)
		if err == nil && block != nil && block.Header != nil {
			stateRoot = block.Header.StateRoot
			timestamp = int64(block.Header.Timestamp)
		}
	} else if len(anchorHash) > 0 {
		block, err := m.blockQuery.GetBlockByHash(ctx, anchorHash)
		if err == nil && block != nil && block.Header != nil {
			blockHash = anchorHash
			height = block.Header.Height
			stateRoot = block.Header.StateRoot
			timestamp = int64(block.Header.Timestamp)
		}
	} else {
		h, bHash, err := m.blockQuery.GetHighestBlock(ctx)
		if err == nil {
			height = h
			blockHash = bHash
			block, err := m.blockQuery.GetBlockByHash(ctx, blockHash)
			if err == nil && block != nil && block.Header != nil {
				stateRoot = block.Header.StateRoot
				timestamp = int64(block.Header.Timestamp)
			}
		}
	}

	// 构造响应
	resp := map[string]interface{}{
		"utxos":  utxoList,
		"height": height,
	}
	if len(blockHash) > 0 {
		resp["block_hash"] = format.HashToHex(blockHash)
	}
	if len(stateRoot) > 0 {
		resp["state_root"] = format.HashToHex(stateRoot)
	}
	if timestamp > 0 {
		resp["timestamp"] = timestamp
	}

	return resp, nil
}

// Call 执行合约调用（只读）
// Method: wes_call
// Params: [callData: object, blockParam: object (optional)]
// callData: {to: contractAddress, data: functionCall, from: callerAddress (optional)}
func (m *StateMethods) Call(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing call data", nil)
	}

	// 1. 解析调用参数
	callData, ok := args[0].(map[string]interface{})
	if !ok {
		return nil, NewInvalidParamsError("callData must be object", nil)
	}

	contractAddr, ok := callData["to"].(string)
	if !ok || contractAddr == "" {
		return nil, NewInvalidParamsError("missing contract address (to)", nil)
	}

	functionData, ok := callData["data"].(string)
	if !ok {
		return nil, NewInvalidParamsError("missing function data", nil)
	}

	// 2. 解析状态锚点参数（可选）
	// 当前实现：wes_call 执行“只读模拟”，不对状态锚点做回放（后续可扩展为按高度/哈希回放）。
	_ = args

	if m.ispcCoordinator == nil {
		return nil, NewInternalError("ISPC coordinator not available", nil)
	}

	// === 3. 解析“to”参数：这里要求为合约 content_hash（32字节）
	//
	// 说明：
	// - WES 的合约“地址”(hash160(content_hash))无法反查回 content_hash；
	// - ISPC 执行入口需要 contractHash（即 content_hash）；
	// 因此 wes_call 的 to 字段在本实现中定义为 content_hash（0x + 64hex 或 64hex）。
	toHex := strings.TrimPrefix(strings.TrimPrefix(contractAddr, "0x"), "0X")
	contractHash, err := hex.DecodeString(toHex)
	if err != nil || len(contractHash) != 32 {
		return nil, NewInvalidParamsError("wes_call requires `to` to be contract content_hash (32 bytes hex), not contract address", nil)
	}

	// === 4. 解析 from（可选）：支持 Base58 WES 地址或 20字节 hex
	callerAddrHex := "0000000000000000000000000000000000000000"
	if fromVal, exists := callData["from"]; exists && fromVal != nil {
		if fromStr, ok := fromVal.(string); ok && fromStr != "" {
			// 1) 20字节hex（0x + 40hex）
			fromHex := strings.TrimPrefix(strings.TrimPrefix(fromStr, "0x"), "0X")
			if raw, decodeErr := hex.DecodeString(fromHex); decodeErr == nil && len(raw) == 20 {
				callerAddrHex = hex.EncodeToString(raw)
			} else {
				// 2) Base58 地址
				if m.addressManager == nil {
					return nil, NewInternalError("address manager not available", nil)
				}
				addr, convErr := m.addressManager.StringToAddress(fromStr)
				if convErr != nil {
					return nil, NewInvalidParamsError(fmt.Sprintf("invalid from address: %v", convErr), nil)
				}
				addrBytes, convErr := m.addressManager.AddressToBytes(addr)
				if convErr != nil {
					return nil, NewInvalidParamsError(fmt.Sprintf("invalid from address: %v", convErr), nil)
				}
				callerAddrHex = hex.EncodeToString(addrBytes)
			}
		}
	}

	// === 5. 解析 data：支持三种形式
	// - 直接方法名："Mint"
	// - JSON 字符串：{"method":"Mint","params":[1,2],"payload":"<base64|0xhex>"}
	// - 0x + hex(JSON bytes)
	type callSpec struct {
		Method  string   `json:"method"`
		Params  []uint64 `json:"params"`
		Payload string   `json:"payload,omitempty"`
	}

	methodName := strings.TrimSpace(functionData)
	var methodParams []uint64
	var payloadBytes []byte

	parsePayload := func(p string) ([]byte, error) {
		if p == "" {
			return nil, nil
		}
		// 0xhex
		if strings.HasPrefix(p, "0x") || strings.HasPrefix(p, "0X") {
			b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(p, "0x"), "0X"))
			if err != nil {
				return nil, err
			}
			return b, nil
		}
		// base64
		return base64.StdEncoding.DecodeString(p)
	}

	tryParseSpec := func(b []byte) bool {
		var spec callSpec
		if err := json.Unmarshal(b, &spec); err != nil {
			return false
		}
		if strings.TrimSpace(spec.Method) == "" {
			return false
		}
		methodName = strings.TrimSpace(spec.Method)
		methodParams = spec.Params
		if pb, err := parsePayload(spec.Payload); err == nil {
			payloadBytes = pb
		} else {
			// payload 提供但无法解析：作为参数错误返回
			payloadBytes = nil
		}
		return true
	}

	// 0xhex(JSON bytes)
	if strings.HasPrefix(methodName, "0x") || strings.HasPrefix(methodName, "0X") {
		if b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(methodName, "0x"), "0X")); err == nil {
			_ = tryParseSpec(b)
		}
	} else {
		// JSON string
		if strings.HasPrefix(strings.TrimSpace(methodName), "{") {
			_ = tryParseSpec([]byte(methodName))
		}
	}

	if methodName == "" {
		return nil, NewInvalidParamsError("missing method name in callData.data", nil)
	}

	// === 6. 调用 ISPC 执行（只读模拟：不构建/签名/提交交易）
	execResult, err := m.ispcCoordinator.ExecuteWASMContract(
		ctx,
		contractHash,
		methodName,
		methodParams,
		payloadBytes,
		callerAddrHex,
	)
	if err != nil {
		m.logger.Warn("wes_call ExecuteWASMContract failed",
			zap.String("contract_hash", hex.EncodeToString(contractHash)),
			zap.String("method", methodName),
			zap.Error(err),
		)
		return nil, NewInternalError(fmt.Sprintf("execute contract: %v", err), nil)
	}

	// 返回结构：尽量对齐"只读调用"的预期，不提交交易，不返回 tx_hash
	resp := map[string]interface{}{
		"success":       true,
		"contract_hash": format.HashToHex(contractHash),
		"method":        methodName,
		"return_values": execResult.ReturnValues,
		"return_data":   hex.EncodeToString(execResult.ReturnData),
		"events":        execResult.Events,
	}
	return resp, nil
}
