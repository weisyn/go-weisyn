#!/usr/bin/env python3
"""
 Genesis密钥对生成工具

用途：生成正确匹配的私钥-公钥-地址三元组，解决CRITICAL-018问题

使用方法：
    python3 scripts/generate_correct_genesis_keys.py
    python3 scripts/generate_correct_genesis_keys.py --count 5
    python3 scripts/generate_correct_genesis_keys.py --output test/genesis_keys_corrected.json
"""

import json
import hashlib
import secrets
from ecdsa import SigningKey, SECP256k1
import argparse
from typing import List, Dict, Any

def keccak256(data: bytes) -> bytes:
    """计算Keccak256哈希"""
    try:
        from Crypto.Hash import keccak
        keccak_hash = keccak.new(digest_bits=256)
        keccak_hash.update(data)
        return keccak_hash.digest()
    except ImportError:
        print("❌ 错误：需要安装 pycryptodome")
        print("请运行：pip3 install pycryptodome")
        exit(1)

def generate_key_pair() -> Dict[str, str]:
    """生成一个正确匹配的密钥对"""
    
    # 1. 生成32字节私钥
    private_key_bytes = secrets.token_bytes(32)
    private_key_hex = private_key_bytes.hex()
    
    # 2. 从私钥推导公钥
    sk = SigningKey.from_string(private_key_bytes, curve=SECP256k1)
    vk = sk.get_verifying_key()
    
    # 获取64字节未压缩公钥（不包含0x04前缀）
    public_key_bytes = vk.to_string()
    public_key_hex = public_key_bytes.hex()
    
    # 3. 从公钥推导地址（Ethereum风格）
    hash_bytes = keccak256(public_key_bytes)
    address_bytes = hash_bytes[12:]  # 取后20字节
    address_hex = address_bytes.hex()
    
    return {
        "private_key": private_key_hex,
        "public_key": public_key_hex,
        "address": address_hex,
        "address_with_prefix": f"0x{address_hex}"
    }

def validate_key_pair(key_pair: Dict[str, str]) -> bool:
    """验证密钥对的正确性"""
    try:
        private_key_bytes = bytes.fromhex(key_pair["private_key"])
        
        # 从私钥重新推导公钥
        sk = SigningKey.from_string(private_key_bytes, curve=SECP256k1)
        vk = sk.get_verifying_key()
        derived_public_key = vk.to_string().hex()
        
        # 从公钥重新推导地址
        public_key_bytes = bytes.fromhex(derived_public_key)
        hash_bytes = keccak256(public_key_bytes)
        derived_address = hash_bytes[12:].hex()
        
        # 验证一致性
        public_key_match = derived_public_key == key_pair["public_key"]
        address_match = derived_address == key_pair["address"]
        
        return public_key_match and address_match
        
    except Exception as e:
        print(f"❌ 验证失败: {e}")
        return False

def generate_genesis_accounts(count: int = 3) -> List[Dict[str, Any]]:
    """生成指定数量的Genesis账户"""
    
    accounts = []
    account_names = [
        "Genesis-A (Primary)",
        "Genesis-B (Secondary)", 
        "Genesis-C (Reserve)",
        "Genesis-D (Testing)",
        "Genesis-E (Development)"
    ]
    
    initial_balances = [
        "1000000000000000000000",  # 1000 wei
        "500000000000000000000",   # 500 wei
        "300000000000000000000",   # 300 wei
        "100000000000000000000",   # 100 wei
        "50000000000000000000"     # 50 wei
    ]
    
    for i in range(count):
        print(f"🔑 生成账户 {i+1}/{count}...")
        
        key_pair = generate_key_pair()
        
        # 验证生成的密钥对
        if not validate_key_pair(key_pair):
            raise Exception(f"生成的密钥对 {i+1} 验证失败")
        
        account = {
            "name": account_names[i] if i < len(account_names) else f"Genesis-{chr(65+i)}",
            "private_key": key_pair["private_key"],
            "public_key": key_pair["public_key"],
            "address": key_pair["address"],
            "address_with_prefix": key_pair["address_with_prefix"],
            "initial_balance": initial_balances[i] if i < len(initial_balances) else "10000000000000000000",
            "address_type": "ethereum",
            "curve": "secp256k1",
            "generated_timestamp": int(__import__('time').time())
        }
        
        accounts.append(account)
        print(f"✅ 账户 {account['name']} 生成成功")
        print(f"   地址: {account['address_with_prefix']}")
        print(f"   余额: {account['initial_balance']} wei")
    
    return accounts

def verify_against_existing_balances(accounts: List[Dict[str, Any]], 
                                   known_addresses: List[str]) -> None:
    """验证生成的账户是否与已知有余额的地址匹配"""
    
    print("\n🔍 验证与已知地址的匹配性...")
    
    generated_addresses = {acc["address"] for acc in accounts}
    generated_addresses_with_prefix = {acc["address_with_prefix"] for acc in accounts}
    
    for known_addr in known_addresses:
        # 移除0x前缀进行比较
        clean_known = known_addr.replace("0x", "").lower()
        
        if clean_known in [addr.lower() for addr in generated_addresses]:
            print(f"✅ 匹配找到: {known_addr}")
        else:
            print(f"⚠️  未匹配: {known_addr}")

def save_genesis_config(accounts: List[Dict[str, Any]], output_file: str) -> None:
    """保存Genesis配置到文件"""
    
    config = {
        "metadata": {
            "version": "1.0",
            "description": "区块链Genesis账户配置",
            "generated_timestamp": int(__import__('time').time()),
            "total_accounts": len(accounts),
            "total_initial_supply": sum(int(acc["initial_balance"]) for acc in accounts),
            "generator": "scripts/generate_correct_genesis_keys.py"
        },
        "genesis": {
            "network_id": "WES_testnet",
            "chain_id": 12345,
            "consensus": "pow",
            "genesis_accounts": accounts
        },
        "validation": {
            "all_key_pairs_verified": True,
            "address_derivation_method": "keccak256_last_20_bytes",
            "public_key_format": "uncompressed_64_bytes",
            "private_key_format": "32_bytes_hex"
        }
    }
    
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(config, f, indent=2, ensure_ascii=False)
    
    print(f"\n💾 配置已保存到: {output_file}")

def main():
    parser = argparse.ArgumentParser(description="生成 Genesis密钥对")
    parser.add_argument("--count", "-c", type=int, default=3, 
                       help="生成的账户数量 (默认: 3)")
    parser.add_argument("--output", "-o", type=str, 
                       default="test/genesis_keys_corrected.json",
                       help="输出文件路径")
    parser.add_argument("--verify", action="store_true",
                       help="验证生成的密钥对")
    
    args = parser.parse_args()
    
    print("🚀WES Genesis密钥对生成工具")
    print("=" * 50)
    
    # 检查依赖
    try:
        import ecdsa
        from Crypto.Hash import keccak
        print("✅ 所有依赖已满足")
    except ImportError as e:
        print(f"❌ 缺少依赖: {e}")
        print("请运行: pip3 install ecdsa pycryptodome")
        return
    
    # 生成账户
    print(f"\n🔑 开始生成 {args.count} 个Genesis账户...")
    accounts = generate_genesis_accounts(args.count)
    
    # 已知有余额的地址（从测试中发现）
    known_addresses = [
        "0xf0fe522b88e267828bbd620207367826cc7b6dfc",
        "0xe77c82a414c2dfef3c2fbfdb92bfa1bbc6283736",
        "0xe470639355a0064ef79079a55570bb6a7171a49a"
    ]
    
    # 验证匹配性
    verify_against_existing_balances(accounts, known_addresses)
    
    # 额外验证
    if args.verify:
        print("\n🔍 执行额外验证...")
        for i, account in enumerate(accounts):
            if validate_key_pair(account):
                print(f"✅ 账户 {i+1} 验证通过")
            else:
                print(f"❌ 账户 {i+1} 验证失败")
    
    # 保存配置
    save_genesis_config(accounts, args.output)
    
    # 显示总结
    print("\n📊 生成总结:")
    print(f"   生成账户数: {len(accounts)}")
    print(f"   总初始供应量: {sum(int(acc['initial_balance']) for acc in accounts)} wei")
    print(f"   配置文件: {args.output}")
    
    print("\n🎯 下一步操作:")
    print("1. 检查生成的配置文件")
    print("2. 更新系统配置以使用新的密钥对")
    print("3. 重新启动节点并测试转账功能")
    print("4. 验证API返回的地址格式")

if __name__ == "__main__":
    main() 