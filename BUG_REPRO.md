# Bug 是什么

配置解码吞掉尾随 JSON 文档的错误，内容校验与差异比较继续把损坏配置当作合法输入。

# 如何触发

在记录基线中执行 `g'o' t'est' ./i'nternal/doma'in/configuration -r'un' '^TestCompareJSONRejectsTrailingDocument$' -coun't'=1`。

# 错误信息

```text
--- FAIL: TestCompareJSONRejectsTrailingDocument (0.00s)
    annotation_test.go:7: trailing JSON document was accepted
FAIL
```
