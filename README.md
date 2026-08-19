# Edge Rollout Control

边缘设备配置发布控制平台，提供纯Go REST API和后台任务。平台管理设备标签与动态设备组、配置版本、兼容性校验、立即/定时/百分比分批发布、设备拉取、幂等回执、风险规则自动暂停、人工恢复/取消/回滚、审计事件和Webhook投递。

## 已实现能力

- 设备注册、编辑、心跳、在线状态与标签筛选。
- `key=value`设备组选择器，可解析组成员。
- JSON/YAML配置校验、SHA-256快照、兼容型号、变量渲染、版本列表与发布/废弃状态。
- 立即、定时和百分比分批发布；设备端拉取待配置并提交成功或失败回执。
- `Idempotency-Key`唯一约束，重复回执返回`created=false`，不会重复推进状态。
- 失败率、超时率和离线率规则；规则命中后自动暂停并记录审计事件。
- 暂停、恢复、取消、回滚状态机；回滚将已处理目标置为回滚待发送。
- Webhook订阅、事件过滤、HMAC签名、失败重试和投递查询。
- 健康、就绪、指标、请求ID、结构化日志、恢复、超时、限流、CORS和统一错误响应。
- `/console/` 运维页面可直接查看控制面健康状态和最近发布任务。

## 目录结构

```text
cmd/edge-rollout/          服务入口
internal/domain/           设备、配置、发布、回执、规则、审计、Webhook领域模型
internal/application/      用例编排、调度、回执处理、Webhook dispatcher和报表
internal/infrastructure/   SQLite连接、迁移和仓储实现
api/                       HTTP handler、路由、中间件、OpenAPI和指标
api/handler/console/       内嵌的边缘运维前端页面
configs/                   YAML配置
migrations/postgres/       PostgreSQL生产迁移参考
deploy/                    Dockerfile和PostgreSQL compose
scripts/                   端到端开发验证脚本
```

## 本地运行

要求Go 1.26+。默认使用`modernc.org/sqlite`，不需要启动外部数据库。

```bash
go mod tidy
go test ./...
go vet ./...
go build ./...
go run ./cmd/edge-rollout -config configs/config.yaml
```

默认监听`:8080`，数据库为`data/edge-rollout.db`。主要环境变量：`SERVER_ADDR`、`DATABASE_DSN`、`LOG_LEVEL`、`WORKER_SCHEDULER_INTERVAL`、`WORKER_WEBHOOK_INTERVAL`、`WORKER_RECEIPT_TIMEOUT`和`WEBHOOK_MAX_ATTEMPTS`。

服务启动后可访问 `http://127.0.0.1:8080/console/` 打开运维页面。前端为内嵌静态资源；`npm run build` 会检查浏览器脚本语法。

## 端到端验证

```bash
./scripts/run-dev.sh
```

脚本使用临时端口`18081`和临时SQLite文件，真实执行启动、健康/就绪检查、设备注册、配置发布、发布启动、设备拉取、成功回执、重复幂等回执、发布完成与审计查询，最后发送`SIGTERM`并清理进程及临时文件。

## API示例

```bash
curl -X POST http://127.0.0.1:8080/v1/devices \
  -H 'Content-Type: application/json' \
  -d '{"name":"pump-01","hardware_model":"pump-v1","software_version":"1.0.0","labels":{"area":"east"}}'

curl -X POST http://127.0.0.1:8080/v1/configurations \
  -H 'Content-Type: application/json' \
  -d '{"name":"pump-config","format":"json","content":"{\"sampling\":30}","compatible_models":["pump-v1"]}'

curl http://127.0.0.1:8080/v1/devices/{device_id}/pending-configuration

curl -X POST http://127.0.0.1:8080/v1/devices/{device_id}/receipts \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: receipt-001' \
  -d '{"rollout_id":"...","configuration_id":"...","status":"succeeded","message":"applied"}'
```

接口索引见`api/openapi.yaml`。

## 数据库与部署

`migrations/postgres/0001_init.sql`包含设备、组、配置、规则、发布目标、回执、审计和Webhook表结构；`deploy/docker-compose.yml`提供PostgreSQL参考实例。当前二进制默认采用SQLite以便本地零依赖验证，仓储接口已隔离数据库实现。

## 验证记录

已执行`gofmt`、`go test ./...`、`go vet ./...`、`go build ./...`和`./scripts/run-dev.sh`。非测试Go源码超过6000行，统计排除测试、生成文件、构建产物、依赖和文档；代码按领域、应用、HTTP和基础设施拆分。
