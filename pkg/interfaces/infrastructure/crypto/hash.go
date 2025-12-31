// Package crypto 提供WES系统的哈希计算接口定义
//
// #️⃣ **哈希计算服务 (Hash Computation Service)**
//
// 本文件定义了WES区块链系统的哈希计算接口，专注于：
// - 多算法支持：SHA256、SHA3、Keccak256、RIPEMD160等主流算法
// - 安全哈希：双重SHA256、HMAC等安全哈希算法
// - 数据校验：数据完整性和一致性校验机制
// - 性能优化：支持流式计算和批量处理
//
// 🎯 **核心功能**
// - HashManager：哈希管理器接口，提供完整的哈希计算服务
// - 算法多样：支持多种主流加密哈希算法
// - 安全强化：双重SHA256、HMAC等安全机制
// - 数据校验：快速的数据完整性验证
//
// 🏧 **设计原则**
// - 算法全面：支持区块链领域常用的所有哈希算法
// - 性能优先：高效的计算实现和内存管理
// - 安全可靠：使用成熟的加密库和算法实现
// - 易用性：统一的接口设计和错误处理
//
// 🔗 **组件关系**
// - HashManager：被区块、交易、Merkle树等模块使用
// - 与MerkleTreeManager：配合进行Merkle树计算
// - 与SignatureManager：提供签名所需的哈希计算
//
// 📖 **完整使用示例**
//
// 1. 小数据一次性哈希：
//
//	// 适用场景：交易ID、区块哈希、Merkle叶子节点等
//	txID := hashMgr.SHA256(txBytes)
//	blockHash := hashMgr.DoubleSHA256(headerBytes)
//
// 2. 大文件流式哈希：
//
//	// 适用场景：资源文件存储、大型合约字节码等
//	hasher := hashMgr.NewSHA256Hasher()
//	file, _ := os.Open("large-contract.wasm")
//	defer file.Close()
//	io.Copy(hasher, file)  // 分块读取，内存友好
//	contentHash := hasher.Sum(nil)
//
// 3. 地址生成（SHA256 + RIPEMD160）：
//
//	// 适用场景：P2PKH地址生成
//	sha := hashMgr.SHA256(publicKeyBytes)
//	addressHash := hashMgr.RIPEMD160(sha)
//
// 4. 比特币兼容双重哈希：
//
//	// 适用场景：区块链共识算法、POW验证
//	powHash := hashMgr.DoubleSHA256(blockHeader)
//
// ⚡ **性能建议**
// - 数据 < 1MB：使用一次性哈希方法（SHA256, DoubleSHA256等）
// - 数据 >= 1MB：使用流式哈希器（NewSHA256Hasher等），避免内存占用
// - 频繁计算相同数据：实现内部缓存（HashManager实现已包含）
package crypto

import "hash"

// HashManager 定义哈希计算相关接口
//
// 提供WES区块链系统的完整哈希计算服务：
// - 多算法支持：SHA256、SHA3、Keccak256、RIPEMD160等算法
// - 安全增强：双重SHA256、HMAC等安全哈希机制
// - 数据校验：快速的数据完整性和一致性验证
// - 格式转换：支持十六进制和字节数组格式
// - 流式哈希：支持大文件的流式哈希计算
type HashManager interface {
	// ==================== 一次性哈希计算 ====================
	//
	// 以下方法适用于小数据量（< 1MB）的场景，一次性将全部数据加载到内存计算哈希。
	// 特点：
	// - ✅ 简单易用，一行代码完成哈希计算
	// - ✅ 内置缓存优化，重复计算相同数据性能更好
	// - ⚠️ 大文件会占用大量内存，建议使用流式方法

	// SHA256 计算SHA-256哈希
	//
	// 适用场景：
	// - 交易ID计算
	// - 区块哈希计算
	// - Merkle叶子节点哈希
	// - 数字签名消息摘要
	//
	// 参数：
	//   - data: 输入数据（建议 < 1MB）
	//
	// 返回：
	//   - []byte: 32字节SHA-256哈希值
	//
	// 示例：
	//   txHash := hashMgr.SHA256(transactionBytes)
	//   fmt.Printf("交易哈希: %x\n", txHash)
	SHA256(data []byte) []byte

	// Keccak256 计算Keccak-256哈希（以太坊兼容）
	//
	// 适用场景：
	// - 以太坊兼容场景
	// - 智能合约事件签名
	// - EVM兼容层地址生成
	//
	// 参数：
	//   - data: 输入数据
	//
	// 返回：
	//   - []byte: 32字节Keccak-256哈希值
	//
	// 示例：
	//   eventSig := hashMgr.Keccak256([]byte("Transfer(address,address,uint256)"))
	Keccak256(data []byte) []byte

	// RIPEMD160 计算RIPEMD-160哈希
	//
	// 适用场景：
	// - 比特币地址生成（SHA256 + RIPEMD160）
	// - P2PKH地址计算
	// - 公钥哈希生成
	//
	// 参数：
	//   - data: 输入数据（通常是SHA256哈希结果）
	//
	// 返回：
	//   - []byte: 20字节RIPEMD-160哈希值
	//
	// 示例：
	//   // 生成比特币风格地址
	//   sha := hashMgr.SHA256(publicKey)
	//   addrHash := hashMgr.RIPEMD160(sha)
	RIPEMD160(data []byte) []byte

	// DoubleSHA256 计算双重SHA-256哈希（比特币兼容）
	//
	// 执行两次SHA256哈希：SHA256(SHA256(data))
	//
	// 适用场景：
	// - 比特币区块哈希
	// - POW工作量证明验证
	// - Merkle根计算
	// - 区块链共识算法
	//
	// 参数：
	//   - data: 输入数据
	//
	// 返回：
	//   - []byte: 32字节双重SHA-256哈希值
	//
	// 示例：
	//   blockHash := hashMgr.DoubleSHA256(blockHeaderBytes)
	//   if bytes.Compare(blockHash, targetDifficulty) < 0 {
	//       fmt.Println("找到有效区块！")
	//   }
	DoubleSHA256(data []byte) []byte

	// ==================== 流式哈希计算 ====================
	//
	// 以下方法适用于大文件（>= 1MB）或流式数据的场景，分块处理避免内存溢出。
	// 特点：
	// - ✅ 内存友好，支持GB级别文件
	// - ✅ 符合标准 hash.Hash 接口，可与 io.Copy 等标准库配合
	// - ✅ 支持分块写入，灵活控制读取节奏
	// - ⚠️ 需要手动调用 Sum() 获取最终哈希

	// NewSHA256Hasher 创建SHA-256流式哈希器
	//
	// 返回标准 hash.Hash 接口，支持分块写入和流式计算。
	// 适用于大文件或流式数据的哈希计算，避免一次性加载全部数据到内存。
	//
	// 适用场景：
	// - 资源文件存储（WASM合约、ONNX模型、大型数据文件）
	// - 大文件内容寻址（基于哈希的去重存储）
	// - 流式数据完整性验证
	// - 分块上传文件的哈希计算
	//
	// 返回：
	//   - hash.Hash: 标准哈希接口，实现 io.Writer
	//
	// 完整示例1 - 大文件哈希计算：
	//
	//   hasher := hashMgr.NewSHA256Hasher()
	//   file, err := os.Open("large-contract.wasm")
	//   if err != nil {
	//       return err
	//   }
	//   defer file.Close()
	//
	//   // 流式读取，自动分块，内存占用恒定
	//   _, err = io.Copy(hasher, file)
	//   if err != nil {
	//       return err
	//   }
	//
	//   // 获取最终哈希
	//   contentHash := hasher.Sum(nil)
	//   fmt.Printf("文件哈希: %x\n", contentHash)
	//
	// 完整示例2 - 分块写入：
	//
	//   hasher := hashMgr.NewSHA256Hasher()
	//   buffer := make([]byte, 4096) // 4KB 缓冲区
	//
	//   for {
	//       n, err := reader.Read(buffer)
	//       if n > 0 {
	//           hasher.Write(buffer[:n])
	//       }
	//       if err == io.EOF {
	//           break
	//       }
	//       if err != nil {
	//           return err
	//       }
	//   }
	//
	//   hash := hasher.Sum(nil)
	//
	// 性能对比：
	//   - 10MB文件：流式哈希内存占用 ~8KB，一次性哈希 ~10MB
	//   - 1GB文件：流式哈希内存占用 ~8KB，一次性哈希 ~1GB（可能OOM）
	NewSHA256Hasher() hash.Hash

	// NewRIPEMD160Hasher 创建RIPEMD-160流式哈希器
	//
	// 返回标准 hash.Hash 接口，支持分块写入和流式计算。
	// 主要用于大数据量的地址生成等场景。
	//
	// 适用场景：
	// - 大批量公钥的地址生成
	// - 流式数据的RIPEMD160哈希
	// - 与SHA256组合使用的比特币风格地址
	//
	// 返回：
	//   - hash.Hash: 标准哈希接口，实现 io.Writer
	//
	// 示例 - 流式地址生成：
	//
	//   // 第一步：SHA256
	//   sha256Hasher := hashMgr.NewSHA256Hasher()
	//   sha256Hasher.Write(largePublicKeyData)
	//   shaResult := sha256Hasher.Sum(nil)
	//
	//   // 第二步：RIPEMD160
	//   ripemdHasher := hashMgr.NewRIPEMD160Hasher()
	//   ripemdHasher.Write(shaResult)
	//   addressHash := ripemdHasher.Sum(nil)
	//
	//   fmt.Printf("地址哈希: %x\n", addressHash)
	NewRIPEMD160Hasher() hash.Hash
}
