# KiteCache

基于 Go 的分布式缓存项目，提供进程内缓存、分片 LRU2 存储、缓存组、节点路由及 gRPC 通信。可以作为本地缓存库使用，也可以通过 etcd 注册发现组成多节点缓存。

## 功能

- **缓存存储**：支持 LRU 与分片双层 LRU2；支持过期时间、定时清理和淘汰回调。
- **缓存组**：通过 `Group` 隔离命名空间，缓存未命中时调用 `Getter` 回源。
- **请求合并**：使用 singleflight 合并同 key 的并发加载；当前实现仍有并发边界待完善，见下方说明。
- **节点路由**：通过一致性哈希与虚拟节点选择目标节点。
- **服务发现**：使用 etcd 租约注册节点，监听成员变化并更新连接与路由。
- **节点通信**：通过 gRPC 执行 Get、Set、Delete；写删操作支持异步向其他节点同步。
- **统计信息**：提供缓存命中、未命中、回源等统计。

## 快速开始

需要 Go 1.22 或更高版本。单机使用及本地压测不需要 etcd；多节点示例需要运行在 `localhost:2379` 的 etcd 3.x。

```bash
git clone https://github.com/chosenlau/KiteCache.git
cd KiteCache
go mod download
go build ./...
```

在业务项目中引入：

```bash
go get github.com/chosenlau/KiteCache
```

```go
package main

import (
    "context"
    "fmt"
    "log"

    kitecache "github.com/chosenlau/KiteCache"
)

func main() {
    group := kitecache.NewGroup("products", 8<<20, kitecache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            // 在这里查询数据库或调用其他数据源。
            return []byte("product:" + key), nil
        },
    ))
    defer group.Close()

    value, err := group.Get(context.Background(), "1001")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(value.String())
}
```

默认使用 LRU2，其容量通过桶数和每桶条目数控制；`MaxBytes` 是 LRU 存储的字节容量配置，不能视为 LRU2 的内存硬限制。可以通过 `WithCacheOptions` 指定存储类型和容量参数，通过 `WithExpiration` 设置缓存组过期时间。

### 多节点示例

先启动 etcd，再在三个终端分别运行：

```bash
go run ./example -port=8001 -node=A
go run ./example -port=8002 -node=B
go run ./example -port=8003 -node=C
```

示例使用服务名 `kite-cache`，会等待节点发现和其他节点准备，然后演示本地写入与跨节点读取。示例中的数据源按节点生成演示值；实际部署应让节点使用一致的数据源。当前没有提供持久化、共识复制或强一致性保证。

## 压测

### 本机实测

测试日期：2026-10-29。环境：Windows amd64、Go 1.26.1、Intel Core i5-10300H @ 2.50GHz（4 个物理核心 / 8 个逻辑处理器）。以下基准使用 `-cpu=8 -benchtime=1s -count=1`，是单次运行结果。

| 场景 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| 默认 LRU2 并发读取 | 384.9 | 15 | 1 |
| 默认 LRU2 混合操作（80% 读 / 20% 写） | 421.5 | 30 | 2 |
| LRU 并发读取 | 1136 | 15 | 1 |
| LRU2 并发读取 | 621.9 | 15 | 1 |

并发读取基准预先写入 10,000 个 key，再随机访问；混合基准预先写入 5,000 个 key，读操作访问这一集合，写操作使用递增的新 key。计时包含 key 格式化、随机数生成和存储路径中的输出等开销。LRU 与 LRU2 的容量策略不同，读取也可能未命中；这组结果不能直接解释为相同命中率下的算法提速。

另一次 **100 个 goroutine、持续 10 秒、70% 读 / 30% 写** 的本地压力测试完成 **836,395 次操作**，吞吐约 **83,634 ops/s**。测试从空缓存开始，在 10,000 个 key 中随机读写，未命中不视为错误。它主要观察混合负载吞吐，不替代正确性验证。

命中率用例先写入 5,000 个 key，再执行 10,000 次读，测得 **100% 命中**；这是数据能够驻留缓存的预热场景，不代表业务命中率或容量压力下的表现。

以上均为**进程内缓存操作**，没有经过 gRPC、etcd 或真实数据库。`ns/op` 是并发基准的总耗时除以操作次数，吞吐倒数也不是请求平均响应延迟；当前用例不统计 P95/P99。结果会受到机器负载、缓存容量、命中率与日志输出影响。

### 复现命令

```bash
# 复现上表；跳过普通测试，避免将压力测试重复计入每个基准命令
go test . -run '^$' -bench 'Benchmark(CacheGet_Parallel|CacheMixed_Parallel|LRU_Get_Parallel|LRU2_Get_Parallel)$' -benchmem -benchtime=1s -count=1 -cpu=8

# 观察不同 GOMAXPROCS 下的结果，重复三次
go test . -run '^$' -bench 'Benchmark(CacheGet|CacheSet|CacheMixed|LRU)' -benchmem -benchtime=3s -count=3 -cpu=1,2,4,8

# 命中率与 100 并发压力测试
go test . -v -run '^(TestCacheHitRate|TestConcurrentStress)$' -count=1 -timeout=60s
```

Windows 可运行 `run_benchmark.bat`，Linux/macOS 可运行 `bash run_benchmark.sh`。脚本将结果保存到本地 `benchmark_results/`，该目录不提交到 Git。

## 目录

```text
.
├── cache.go             # 统一缓存接口及统计
├── group.go             # 缓存组、回源和写删同步
├── client.go / server.go # gRPC 客户端和服务端
├── peers.go             # 节点选择与服务发现
├── store/               # LRU、分片双层 LRU2 和存储测试
├── consistenthash/      # 一致性哈希
├── singleflight/        # 同 key 请求合并
├── registry/            # etcd 注册与租约
├── pb/                  # Protobuf 定义和生成代码
├── example/             # 多节点演示
└── benchmark_test.go    # 本地基准及压力测试
```

## 验证与当前限制

```bash
go build ./...
go test ./... -count=1 -timeout=60s
```

发布整理时各包编译通过，本地命中率与压力测试通过；完整测试中的 `TestLRU2StoreLRUEviction`、`TestLRU2StoreHitRatio` 未通过，仍需核对双层容量及淘汰语义。当前 singleflight 的检查和注册不是原子操作，且没有完整处理回源 panic；因此不能承诺所有并发场景下同 key 只回源一次。上述限制尚未修复，多节点示例也未在本次发布中进行集群联调。

### 更新协议代码

使用 protoc 29.3、protoc-gen-go v1.36.4、protoc-gen-go-grpc v1.5.1，将工具加入 PATH 后运行：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.4
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pb/kite.proto
```

## 许可证

采用 [MIT License](LICENSE)。代码包含上游衍生部分，许可证中保留了原始版权与许可声明。
