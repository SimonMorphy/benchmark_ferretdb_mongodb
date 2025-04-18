# FerretDB vs MongoDB 性能基准测试工具

这是一个用于比较 FerretDB 和 MongoDB 性能的基准测试工具。该工具支持多种测试场景，包括基本的 CRUD 操作、并发测试和复杂查询测试。

## 环境要求

- Go 1.21 或更高版本
- Docker 和 Docker Compose
- FerretDB 2.1.0 或更高版本
- MongoDB 7.0 或更高版本

## 快速开始

1. 启动数据库服务

```bash
docker-compose up -d
```

2. 安装依赖

```bash
go mod download
```

3. 运行基准测试

基本用法：

```bash
go run main.go
```

快速测试（跳过预热阶段）：

```bash
go run main.go -no-warmup
```

自定义参数：

```bash
go run main.go -concurrent 50 -docsize 10240 -duration 10m -op insert -no-warmup
```

## 可用参数

- `-ferretdb`: FerretDB 连接 URI (默认: "mongodb://root:password@localhost:27017")
- `-mongodb`: MongoDB 连接 URI (默认: "mongodb://root:password@localhost:27018")
- `-db`: 数据库名称 (默认: "benchmark")
- `-collection`: 集合名称 (默认: "test")
- `-concurrent`: 并发连接数 (默认: 10)
- `-docsize`: 文档大小（字节）(默认: 1024)
- `-duration`: 测试持续时间 (默认: 5m)
- `-op`: 操作类型 (insert/query/update/delete) (默认: "insert")
- `-rw`: 读写比例 (0.7 表示 70% 读操作) (默认: 0.7)
- `-no-warmup`: 跳过预热阶段 (默认: false，设置此标志将跳过 5 分钟预热)

## 测试场景示例

1. 基础 CRUD 测试（快速模式）
   
   ```bash
   # 插入测试 (1KB 文档)
   go run main.go -op insert -docsize 1024 -concurrent 10 -no-warmup
   ```

# 查询测试

go run main.go -op query -concurrent 50 -no-warmup

# 更新测试

go run main.go -op update -concurrent 20 -no-warmup

# 删除测试

go run main.go -op delete -concurrent 10 -no-warmup

```
2. 并发负载测试（完整模式）
```bash
# 50 并发连接
go run main.go -concurrent 50 -duration 10m

# 100 并发连接
go run main.go -concurrent 100 -duration 10m

# 500 并发连接
go run main.go -concurrent 500 -duration 10m
```

3. 文档大小测试（快速模式）
   
   ```bash
   # 10KB 文档
   go run main.go -docsize 10240 -no-warmup
   ```

# 100KB 文档

go run main.go -docsize 102400 -no-warmup

```

## 测试结果

测试结果将以 JSON 格式保存在当前目录下，文件名格式为 `results_<operation>_<timestamp>.json`。
同时，测试过程中的实时结果会打印到控制台。

结果包含以下指标：

- 预热状态（是否跳过）
- 每秒操作数 (Operations/sec)
- 平均延迟 (Average Latency)
- P95 延迟 (95th Percentile Latency)
- P99 延迟 (99th Percentile Latency)
- 错误数 (Error Count)
- 测试持续时间 (Time Elapsed)

## 注意事项

1. 在运行测试前，确保：
   
   - 数据库服务正常运行
   - 有足够的磁盘空间
   - 系统资源未被其他进程大量占用

2. 测试过程中：
   
   - 默认每个测试会进行 5 分钟预热（可通过 -no-warmup 跳过）
   - 建议在专用环境中进行测试
   - 避免在生产环境运行大规模测试

3. 资源限制：
   
   - 注意监控系统资源使用情况
   - 大规模测试可能需要调整系统限制（如最大文件描述符数量）

4. 预热说明：
   
   - 快速测试：使用 -no-warmup 跳过预热，适合开发测试
   - 完整测试：不使用 -no-warmup，包含预热阶段，适合性能基准测试