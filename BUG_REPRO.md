# Bug 是什么

审计时间线消费失败时，defer 没有清理错误分支，事件、资源标识与时间仍留在半成品对象中。

# 如何触发

在记录基线中执行 `go 'test' ./int'ernal/dom'ain/audit -run '^TestBuildTimelineAndRunCleansFailedTimeline$' -count=1`。

# 错误信息

```text
--- FAIL: TestBuildTimelineAndRunCleansFailedTimeline (0.00s)
    annotation_test.go:14: failed timeline retained state: err=stop timeline=&audit.Timeline{ResourceType:"rollout", ResourceID:"r", Events:[]audit.Event{...}}
FAIL
```
