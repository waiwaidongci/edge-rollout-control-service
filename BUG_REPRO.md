# Bug 是什么

目标状态转换的 nil 路径契约不一致：写入和读取入口发生 nil 解引用，另外两个入口静默接受缺失目标。

# 如何触发

在记录基线中逐条执行 collection.json 的四条 `verify_cmds`，其中状态写入命令会直接触发 panic。

# 错误信息

```text
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
[signal SIGSEGV: segmentation violation code=0x2 addr=0x68]
github.com/example/edge-rollout-control/internal/domain/rollout.ApplyTargetTransition(...)
    internal/domain/rollout/rollout.go:106
FAIL
```
