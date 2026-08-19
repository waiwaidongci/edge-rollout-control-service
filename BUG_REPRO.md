# Bug 是什么

回执排序和派生快照共享调用者的切片底层数组与时间指针，排序或修改快照会污染原数据。

# 如何触发

在记录基线中执行 `go test -'run' '^TestSortByReceivedAtDoesNotMutateInput$' -cou'nt'=1 ./inte'rnal/domain/receipt'`。

# 错误信息

```text
--- FAIL: TestSortByReceivedAtDoesNotMutateInput (0.00s)
    annotation_test.go:14: input order changed: []receipt.Receipt{receipt.Receipt{ID:"new"}, receipt.Receipt{ID:"old"}}
FAIL
```
