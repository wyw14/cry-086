# 塔安智控 Crane Guard

塔安智控是一个完全离线可运行的智慧工地塔机安全监测平台。它从本地设备模拟器接收载重、幅度、高度、回转角、风速和限位状态，将原始遥测绑定到塔机型号、传感器校准版本和当前安全配置版本，完成数据质量判定、风险联锁、告警处置、群塔碰撞预测、设备保障、事件回放和监管报告。

## 核心业务

- 工地、责任单位、司机、塔机型号、安装坐标和运行状态统一登记。
- 传感器量程、测点、单位、校准版本和通信状态独立管理。
- 遥测接入执行单位归一、事件去重、有限乱序接纳和坏数据隔离。
- 安全判定使用精确的型号与配置版本，数据不足时明确返回 `degraded`，不会伪造正常。
- 高风险事件可提出联锁建议；相同遥测事件只创建一个告警。解除联锁必须先确认、满足恢复保持期并由有权人员操作。
- 群塔模块在本地坐标系中预测吊钩轨迹，同时考虑水平与垂直安全间距。
- 巡检、维保、校准、故障、停机和复机使用集中状态机与乐观版本。
- 原始遥测、告警证据和审计记录不可覆盖；监管报告引用证据 ID 和内容摘要。

## 架构与目录

```text
cmd/server                 HTTP 服务入口
cmd/simulator              本地设备模拟器
cmd/migrate                PostgreSQL 迁移入口
internal/domain            领域实体、值对象、状态机和安全策略
internal/application       遥测、安全、告警、群塔、维保、报告等用例
internal/repository        内存演示仓储与 pgx PostgreSQL 仓储
internal/transport/http    Gin 路由、参数绑定和稳定错误响应
internal/middleware        request_id、认证、限流、安全头和恢复
internal/platform          时钟、ID、幂等、文件、outbox 和本地通知适配器
migrations                 版本化 PostgreSQL schema
api/openapi                OpenAPI 3.0 文档
web                        Vue 3 + TypeScript + Vite + Pinia + Element Plus
tests/integration          PostgreSQL 仓储集成测试
deploy                     部署说明
```

接口靠近应用层调用方定义，依赖由构造函数注入；时钟、ID、事务仓储、文件和通知适配器均可替换。所有 I/O 方法接收 `context.Context`。后台 outbox 支持取消、指数退避、最大重试和死信。

## 本地启动

需要 Go 1.24+ 和 Node.js 22+。

```bash
cp .env.example .env
go run ./cmd/server
```

默认使用内存仓储和确定性演示数据，不需要联网或 PostgreSQL。前端开发模式：

```bash
cd web
npm ci
npm run dev
```

浏览器打开 `http://localhost:5173`。演示账号为 `safety`，演示密码为 `CraneGuard!2026`。该凭据只存在于本地种子数据中，生产部署必须关闭 `SEED_DEMO`。

## 容器启动与迁移

```bash
docker compose up --build
```

Compose 会启动应用和 PostgreSQL。应用容器在启动服务前幂等执行 `migrations/000001_initial.up.sql`，使用非 root 用户 UID 10001。Dockerfile 通过 `TARGETOS`/`TARGETARCH` 支持 amd64 和 arm64。

手工迁移：

```bash
DATABASE_URL='postgres://crane_guard:crane_guard@localhost:5432/crane_guard?sslmode=disable' go run ./cmd/migrate
```

## 设备模拟

服务启动后运行：

```bash
go run ./cmd/simulator -endpoint http://localhost:8080/api/v1/telemetry -key local-simulator-key -crane crane-a
```

模拟器只调用本地 HTTP 接口，数据和调度完全确定，不依赖云服务、地图、短信或推送平台。

## API 示例

```bash
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"safety","password":"CraneGuard!2026"}'
```

所有业务 API 位于 `/api/v1`。错误响应始终包含稳定 `code`、可读 `message`、`field_errors` 和 `request_id`。列表接口使用游标或受限页大小，并只允许文档列出的排序与筛选字段。完整接口见 `api/openapi/openapi.yaml`。

## 状态机

- 塔机：`commissioning -> running -> stopped/faulted -> stopped -> running/retired`，禁止从运行态直接退役。
- 告警：`open -> acknowledged -> recovering -> resolved`；人工接管是受权操作，不能替代恢复条件。
- 工单：`planned -> in_progress -> completed/failed`；校准和故障修复完成时必须提交受控文件证据。
- 停复机：完成维保后先停机，安全员完成复核后才能产生补偿式复机事件。

## 时间、审计与安全

后端以 UTC 保存时间，前端按浏览器和工地配置时区展示。审计记录包含操作者、来源、前后差异、原因、request_id 和时间，并使用哈希检测篡改。短期访问令牌配合可撤销、轮换的刷新令牌；密码使用 bcrypt。RBAC 与工地资源归属都在服务端校验。日志不会记录密码、令牌、签名正文或文件内容。

证据文件校验大小、扩展名、MIME 和 SHA-256，并存入工地隔离路径；下载只能走授权 API，响应不暴露服务器真实路径。

## 测试与验证

```bash
gofmt -w $(find cmd internal tests -name '*.go')
go build ./...
go test ./...
go test -race ./...
go vet ./...
cd web && npm ci && npm test && npm run build
```

PostgreSQL 集成测试默认跳过。准备已迁移的隔离数据库后设置 `TEST_DATABASE_URL` 再运行 `go test ./tests/integration`。

已执行验证覆盖领域状态机、配置版本、安全降级、遥测去重、审计不可变、恢复授权、群塔空间关系、幂等并发、outbox 死信、HTTP 参数/认证/资源归属、前端风险状态和生产构建。

## 限制

本项目只提供本地通知适配器和本地轨迹模拟，不连接真实短信、推送、地图或塔机控制器。联锁结果是安全建议，实际机械控制必须由经过认证的现场控制系统执行。示例 HMAC 访问令牌适用于离线演示；多实例生产部署应替换为集中密钥管理和标准身份提供方。
