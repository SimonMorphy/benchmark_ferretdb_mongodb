# FerretDB vs MongoDB 性能对比测试报告

## 一、基础配置说明

### 容器资源配置

```yaml
version: '3.8'

services:
  postgres:
    image: ghcr.io/ferretdb/postgres-documentdb:17-0.102.0-ferretdb-2.1.0
    platform: linux/amd64
    restart: on-failure
    environment:
      - POSTGRES_USER=root
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=postgres
    volumes:
      - ./data/ferretdb/postgres:/var/lib/postgresql/data
    networks:
      - ferretdb
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G

  ferretdb:
    image: ghcr.io/ferretdb/ferretdb:2.1.0
    restart: on-failure
    ports:
      - 27017:27017
    environment:
      - FERRETDB_POSTGRESQL_URL=postgres://root:password@postgres:5432/postgres
    depends_on:
      - postgres
    networks:
      - ferretdb
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G

  mongodb:
    image: mongo:7.0
    platform: linux/amd64
    restart: on-failure
    ports:
      - 27018:27017
    environment:
      - MONGO_INITDB_ROOT_USERNAME=root
      - MONGO_INITDB_ROOT_PASSWORD=password
    volumes:
      - ./data/mongodb:/data/db
    networks:
      - ferretdb
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G

networks:
  ferretdb:
    name: ferretdb
```

### 数据持久化路径

推荐的宿主机挂载目录结构：

```
/data/
  ├── ferretdb/
  │   ├── postgres/       # PostgreSQL 数据目录
  │   └── mongodb/        # MongoDB 数据目录
  └── benchmark/
      ├── results/        # 测试结果输出
      └── datasets/       # 测试数据集
```

### 测试工具清单

1. YCSB (必选)
   
   - 版本：0.17.0
   - MongoDB 绑定：mongodb-binding-0.17.0
   - 用途：标准化负载测试

2. mongobench (必选)
   
   - 自定义 Go 实现
   - 用途：细粒度 CRUD 性能测试

3. 原生监控命令
   
   - db.serverStatus()
   - db.stats()
   - explain() 执行计划分析

4. go-testing (可选)
   
   - 用途：单元测试级别性能分析

## 二、测试用例设计

### 2.1 基础性能测试（单线程）

#### 文档大小测试矩阵

| 操作类型 | 文档大小  | 索引字段 | 非索引字段 |
| ---- | ----- | ---- | ----- |
| 插入   | 1KB   | -    | -     |
| 查询   | 1KB   | ✓    | ✓     |
| 更新   | 1KB   | ✓    | ✓     |
| 删除   | 1KB   | ✓    | -     |
| 插入   | 10KB  | -    | -     |
| 查询   | 10KB  | ✓    | ✓     |
| 更新   | 10KB  | ✓    | ✓     |
| 删除   | 10KB  | ✓    | -     |
| 插入   | 100KB | -    | -     |
| 查询   | 100KB | ✓    | ✓     |
| 更新   | 100KB | ✓    | ✓     |
| 删除   | 100KB | ✓    | -     |

### 2.2 并发负载测试

#### 连接数梯度测试

- 并发连接数：10, 50, 100, 200, 500
- 读写比例：70% 读 / 30% 写
- 一致性配置：
  - readConcern: local
  - writeConcern: majority

#### 事务支持测试

- 单文档事务
- 多文档事务（如果 FerretDB 支持）
- TPS 测试（每秒事务数）

### 2.3 复杂查询能力验证

#### 聚合管道测试

```javascript
db.collection.aggregate([
  { $group: { _id: "$field", count: { $sum: 1 } } },
  { $lookup: {
      from: "other_collection",
      localField: "_id",
      foreignField: "ref_field",
      as: "joined_docs"
    }
  },
  { $sort: { count: -1 } }
])
```

#### 地理空间查询测试

```javascript
db.collection.createIndex({ location: "2dsphere" })
db.collection.find({
  location: {
    $near: {
      $geometry: {
        type: "Point",
        coordinates: [ -73.9667, 40.78 ]
      },
      $maxDistance: 5000
    }
  }
})
```

### 2.4 数据规模扩展性测试

| 数据量级    | 测试项目           |
| ------- | -------------- |
| 10万文档   | 写入耗时、索引构建、空间占用 |
| 100万文档  | 写入耗时、索引构建、空间占用 |
| 1000万文档 | 写入耗时、索引构建、空间占用 |

## 三、指标收集与分析

### 3.1 数据库级指标

```javascript
// 每5秒采集一次
db.serverStatus()
```

关键指标：

- opcounters
- lockStats
- memory.bytes.resident

### 3.2 系统级指标

使用 dstat 采集：

```bash
dstat --cpu --mem --disk --net --output stats.csv 5
```

## 四、测试结果

    link

### 4.1 基础性能测试结果

| 操作类型 | 文档大小 | FerretDB (ops/s) | MongoDB (ops/s) | 性能比率 |
| ---- | ---- | ---------------- | --------------- | ---- |
| 插入   | 1KB  | TBD ± SD         | TBD ± SD        | TBD  |
| 查询   | 1KB  | TBD ± SD         | TBD ± SD        | TBD  |
| 更新   | 1KB  | TBD ± SD         | TBD ± SD        | TBD  |
| 删除   | 1KB  | TBD ± SD         | TBD ± SD        | TBD  |

### 4.2 并发性能测试结果

| 并发数 | 指标类型 | FerretDB | MongoDB | 差异率 |
| --- | ---- | -------- | ------- | --- |
| 10  | TPS  | TBD      | TBD     | TBD |
| 50  | TPS  | TBD      | TBD     | TBD |
| 100 | TPS  | TBD      | TBD     | TBD |

### 4.3 复杂查询性能

| 查询类型 | FerretDB (ms) | MongoDB (ms) | 性能比率 |
| ---- | ------------- | ------------ | ---- |
| 聚合   | TBD           | TBD          | TBD  |
| 地理查询 | TBD           | TBD          | TBD  |

## 五、执行注意事项

1. 预热要求
   
   - 每个测试用例前预留 5 分钟空载运行
   - 清理缓存：`echo 3 > /proc/sys/vm/drop_caches`

2. 测试重复
   
   - 每组参数重复测试 3 次
   - 剔除异常值，取平均值

3. FerretDB 兼容性限制
   
   - 事务支持：仅支持单文档事务
   - 地理空间索引：部分支持
   - 聚合管道：部分操作符可能不支持

4. 测试环境要求
   
   - 独立物理机执行
   - 避免其他负载干扰
   - 网络延迟监控 