# Bug 是什么

应用层包装发布冲突错误后，原始哨兵错误链丢失，调用方无法用 `errors.Is` 识别冲突，错误分类也落入 internal。

# 如何触发

在记录基线中执行 `go test ./internal/application -run '^TestWrapServiceErrorPreservesSentinel$' -count=1`。

# 错误信息

```text
--- FAIL: TestWrapServiceErrorPreservesSentinel (0.00s)
    annotation_test.go:11: sentinel was lost: create rollout: resource conflict
FAIL
```
