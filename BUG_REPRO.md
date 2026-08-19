# Bug 是什么

Webhook 投递失败后增加了重试次数和时间，但状态仍为 pending，重试查询与事件准入得到相互冲突的状态。

# 如何触发

在记录基线中执行 `go test -count=1 ./inter'nal/domain'/webhook -run '^TestScheduleRetryPublishesRetryingState$'`。

# 错误信息

```text
--- FAIL: TestScheduleRetryPublishesRetryingState (0.00s)
    annotation_test.go:14: unexpected retry state: webhook.Delivery{Status:"pending", Attempts:1, LastError:"temporary"}
FAIL
```
