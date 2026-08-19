# Bug 是什么

指标写入与四个快照读取入口并发访问同一共享 map，读锁范围缺失会产生 data race 与不一致视图。

# 如何触发

在记录基线中执行 `go test -race ./api -run '^TestMetricRegistryConcurrentSnapshot$' -count=1`。

# 错误信息

```text
WARNING: DATA RACE
Write at 0x00c000115140 by goroutine 12:
  github.com/example/edge-rollout-control/api.(*MetricRegistry).Add()
      api/metrics.go:95
Previous read at 0x00c000115140 by goroutine 11:
  github.com/example/edge-rollout-control/api.(*MetricRegistry).Snapshot()
      api/metrics.go:207
FAIL
```
