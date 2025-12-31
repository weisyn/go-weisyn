# API 文档（internal/api/docs）

> **📌 模块类型**：`[ ] 实现模块` `[ ] 接口定义` `[ ] 数据结构` `[X] 工具/其他`

---

## 📍 **模块定位**

　　本目录包含 WES 区块链节点 API 的完整文档规范,包括 OpenAPI 规范和 JSON-RPC 规范。

**包含文档**:
- **openapi.yaml** - REST API 的 OpenAPI 3.0 规范
- **jsonrpc_spec.md** - JSON-RPC API 的完整方法文档
- **README.md** - 本文档

---

## 📚 **文档列表**

### **OpenAPI 规范**
- 文件: [openapi.yaml](./openapi.yaml)
- 用途: REST API 规范,可用于生成 Swagger UI
- 工具: 
```bash
  # 使用 Swagger UI 查看
  docker run -p 28680:28680 -e SWAGGER_JSON=/docs/openapi.yaml \
    -v $(pwd):/docs swaggerapi/swagger-ui
  ```

### **JSON-RPC 规范**
- 文件: [jsonrpc_spec.md](./jsonrpc_spec.md)
- 用途: JSON-RPC 方法完整文档
- 对标: Geth JSON-RPC, EIP-1898

---

## 🔧 **文档维护**

### **更新 OpenAPI**
当添加新的 REST 端点时:
1. 在 `openapi.yaml` 中添加路径定义
2. 添加 schema 定义
3. 添加示例
4. 更新版本号

### **更新 JSON-RPC 规范**
当添加新的 JSON-RPC 方法时:
1. 在 `jsonrpc_spec.md` 中添加方法说明
2. 添加请求/响应示例
3. 说明错误码
4. 更新方法列表

---

## 📋 **待办事项**

- [ ] 完善 OpenAPI 规范中的所有端点
- [ ] 添加 GraphQL schema 文档
- [ ] 生成客户端 SDK 文档
- [ ] 添加 Postman Collection

---

> 📝 **文档说明**  
> 保持文档与代码同步更新!
