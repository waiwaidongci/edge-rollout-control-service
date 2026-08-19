# Bug 是什么

设备标签的构造、补全、克隆与可写转换传播 nil map，零值设备写入标签时 panic，非空输入还可能保留别名。

# 如何触发

在记录基线中执行 collection.json 的组合 `verify_cmds`，四个子场景会覆盖 nil 克隆、构造、可写转换和零值设备补全。

# 错误信息

```text
--- FAIL: TestDeviceLabelNilContracts/clone_nil_and_isolate_values
--- FAIL: TestDeviceLabelNilContracts/new_device_labels
--- FAIL: TestDeviceLabelNilContracts/writable_labels
panic: assignment to entry in nil map [recovered, repanicked]
    internal/domain/device/annotation_test.go:32
testing.tRunner(...)
FAIL
```
