# Bug 是什么

请求中间件与路由改用 background 执行下游工作，并把取消错误吞成 nil，请求结束后工作仍可能继续。

# 如何触发

在记录基线中逐条执行 collection.json 的两条 `verify_cmds`，分别覆盖中间件和路由入口。

# 错误信息

```text
--- FAIL: TestRunWithRequestContextStopsOnClientAbort (0.00s)
    annotation_test.go:17: unexpected cancellation: <nil>
--- FAIL: TestRunRouterWorkStopsOnClientAbort (0.00s)
    annotation_test.go:14: unexpected cancellation: <nil>
FAIL
```
