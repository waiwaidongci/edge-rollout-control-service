# Bug 是什么

仓储层把已取消的调用上下文替换为 background，上游取消和截止时间无法传播到数据库操作。

# 如何触发

在记录基线中执行 `go test -run '^TestOperationContextKeepsCancellation$' ./internal/infrastructure/repository -count=1`。

# 错误信息

```text
--- FAIL: TestOperationContextKeepsCancellation (0.00s)
    annotation_test.go:12: cancelled context was detached
FAIL
```
