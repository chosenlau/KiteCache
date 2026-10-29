package kitecache

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkCacheGet_SingleThread 单线程读取性能
func BenchmarkCacheGet_SingleThread(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.MaxBytes = 100 * 1024 * 1024 // 100MB
	cache := NewCache(opts)
	defer cache.Close()

	// 预热缓存：写入1000个key
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
		cache.Add(key, value)
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		cache.Get(ctx, key)
	}
}

// BenchmarkCacheSet_SingleThread 单线程写入性能
func BenchmarkCacheSet_SingleThread(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.MaxBytes = 100 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
		cache.Add(key, value)
	}
}

// BenchmarkCacheGet_Parallel 并发读取性能（模拟高并发场景）
func BenchmarkCacheGet_Parallel(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.MaxBytes = 100 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	// 预热缓存
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d_with_some_longer_content_to_simulate_real_data", i))}
		cache.Add(key, value)
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			key := fmt.Sprintf("key_%d", r.Intn(10000))
			cache.Get(ctx, key)
		}
	})
}

// BenchmarkCacheSet_Parallel 并发写入性能
func BenchmarkCacheSet_Parallel(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.MaxBytes = 100 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	b.ResetTimer()
	b.ReportAllocs()

	var counter int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1)
			key := fmt.Sprintf("key_%d", i)
			value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
			cache.Add(key, value)
		}
	})
}

// BenchmarkCacheMixed_Parallel 混合读写场景（80%读 + 20%写）
func BenchmarkCacheMixed_Parallel(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.MaxBytes = 100 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	// 预热
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
		cache.Add(key, value)
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	var writeCounter int64 = 5000
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			if r.Intn(100) < 80 { // 80% 读操作
				key := fmt.Sprintf("key_%d", r.Intn(5000))
				cache.Get(ctx, key)
			} else { // 20% 写操作
				i := atomic.AddInt64(&writeCounter, 1)
				key := fmt.Sprintf("key_%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
				cache.Add(key, value)
			}
		}
	})
}

// BenchmarkLRU_vs_LRU2 对比 LRU 和 LRU2 性能
func BenchmarkLRU_Get_Parallel(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.CacheType = "lru"
	opts.MaxBytes = 50 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
		cache.Add(key, value)
	}

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			key := fmt.Sprintf("key_%d", r.Intn(10000))
			cache.Get(ctx, key)
		}
	})
}

func BenchmarkLRU2_Get_Parallel(b *testing.B) {
	opts := DefaultCacheOptions()
	opts.CacheType = "lru2"
	opts.MaxBytes = 50 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d", i))}
		cache.Add(key, value)
	}

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			key := fmt.Sprintf("key_%d", r.Intn(10000))
			cache.Get(ctx, key)
		}
	})
}

// BenchmarkSingleflight 测试 Singleflight 防击穿效果
func BenchmarkSingleflight_WithCache(b *testing.B) {
	// 模拟缓存穿透场景：1000个并发请求同一个不存在的key
	group := NewGroup("test", 10*1024*1024, GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			time.Sleep(10 * time.Millisecond) // 模拟数据库查询延迟
			return []byte("value_from_db"), nil
		}),
	)
	defer group.Close()

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			group.Get(ctx, "hot_key_missing") // 所有请求同一个key
		}
	})
}

// TestCacheHitRate 测试缓存命中率（不是benchmark，但输出重要指标）
func TestCacheHitRate(t *testing.T) {
	opts := DefaultCacheOptions()
	opts.MaxBytes = 10 * 1024 * 1024 // 10MB，故意设小触发淘汰
	cache := NewCache(opts)
	defer cache.Close()

	ctx := context.Background()

	// 写入5000个key
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value_%d_with_more_content_to_fill_cache", i))}
		cache.Add(key, value)
	}

	// 模拟热点访问：前1000个key访问频率高
	totalRequests := 10000
	for i := 0; i < totalRequests; i++ {
		var key string
		if i%10 < 8 { // 80%的请求访问热点数据
			key = fmt.Sprintf("key_%d", rand.Intn(1000))
		} else { // 20%访问冷数据
			key = fmt.Sprintf("key_%d", rand.Intn(5000))
		}
		cache.Get(ctx, key)
	}

	stats := cache.Stats()
	t.Logf("缓存统计信息: %+v", stats)

	if hitRate, ok := stats["hit_rate"].(float64); ok {
		t.Logf("缓存命中率: %.2f%%", hitRate*100)
	}
}

// TestConcurrentStress 压力测试：模拟真实高并发场景
func TestConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（使用 -short 标志）")
	}

	opts := DefaultCacheOptions()
	opts.MaxBytes = 50 * 1024 * 1024
	cache := NewCache(opts)
	defer cache.Close()

	ctx := context.Background()
	duration := 10 * time.Second
	concurrency := 100 // 100个并发协程

	var ops int64
	done := make(chan bool)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			for time.Since(startTime) < duration {
				key := fmt.Sprintf("key_%d", r.Intn(10000))

				if r.Intn(100) < 70 { // 70% 读
					if _, ok := cache.Get(ctx, key); !ok {
						// 未命中不算错误
					}
				} else { // 30% 写
					value := ByteView{b: []byte(fmt.Sprintf("value_%d", r.Int()))}
					cache.Add(key, value)
				}

				atomic.AddInt64(&ops, 1)
			}
			done <- true
		}(i)
	}

	// 等待所有协程完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	elapsed := time.Since(startTime)
	totalOps := atomic.LoadInt64(&ops)
	qps := float64(totalOps) / elapsed.Seconds()

	t.Logf("\n===== 压力测试结果 =====")
	t.Logf("测试时长: %.2f 秒", elapsed.Seconds())
	t.Logf("并发数: %d", concurrency)
	t.Logf("总操作数: %d", totalOps)
	t.Logf("本地缓存吞吐: %.0f ops/sec", qps)
	t.Logf("吞吐倒数（非请求延迟）: %.2f µs/op", elapsed.Seconds()*1e6/float64(totalOps))

	stats := cache.Stats()
	t.Logf("\n缓存统计: %+v", stats)
}

// BenchmarkConsistentHash 一致性哈希性能
func BenchmarkConsistentHash(b *testing.B) {
	// 需要导入 consistenthash 包，这里简化测试
	// 实际应该测试节点映射的查找速度
	b.Skip("需要单独测试一致性哈希模块")
}
