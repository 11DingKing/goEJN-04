# 额济纳旗绿色微电网调度后端

Go 后端服务，为额济纳旗绿色微电网提供调度值班、储能巡检、运维抢修和黑启动并网的全流程数字化支撑。

## 核心业务

围绕三类核心实体运转：

- **储能电池舱** — 单体温度超 45 °C 立即告警并冻结充电；SOC 低于 15 % 禁止离网运行
- **风电光伏场站** — 发电状态监控与异常上报
- **柴油应急发电机组** — 黑启动电源与应急供电

四条工作流：

1. **巡检** — 创建巡检任务，记录结果；发现异常自动创建维修工单
2. **异常上报** — 直接上报设备异常，自动评估温度/SOC 规则并派发工单
3. **维修闭环** — 工单派发 → 接单（15 分钟超时升级班长）→ 维修中 → 异常恶化自动追加关联工单并通知备用抢修队 → 完成
4. **黑启动与自动同期并网** — 停电后 20 分钟内恢复母线电压 → 并网申请 → 调度与运维双方复核签字 → 自动同期并网 → 完成；网络中断时短信报文重发确保指令不丢失

## 项目结构

```
cmd/server/main.go              入口
internal/config/                配置加载（JSON + 环境变量）
internal/domain/                领域模型与状态机（实体、工单、黑启动、并网、负荷、短信）
internal/store/                 线程安全内存持久化
internal/app/                   应用编排（巡检、维修、黑启动、短信重发）
internal/scheduler/             后台任务（超时升级、截止检查、短信重试）
internal/transport/http/        HTTP 接入层
```

- 生产包：6 个（config、domain、store、app、scheduler、transport/http）
- 生产 Go 文件：20 个
- 测试文件：6 个，测试函数：51 个

## 启动

### 本地运行

```bash
go run ./cmd/server
```

服务默认监听 `0.0.0.0:57579`。

### Docker

```bash
# 构建（支持 amd64 与 arm64）
docker build -t ejina-grid:latest .

# 运行
docker run -p 57579:57579 ejina-grid:latest

# 多架构构建
docker buildx build --platform linux/amd64,linux/arm64 -t ejina-grid:latest .
```

## 配置

默认配置文件为 `config.json`，可通过 `GRID_CONFIG_PATH` 环境变量指定路径。支持的环境变量覆盖：

| 环境变量 | 说明 | 默认值 |
|---|---|---|
| `GRID_PORT` | 监听端口 | 57579 |
| `GRID_BATTERY_TEMP_THRESHOLD` | 电池温度阈值 (°C) | 45.0 |
| `GRID_MIN_SOC` | 离网最低 SOC (%) | 15.0 |
| `GRID_WO_ACCEPT_TIMEOUT` | 工单接单超时 | 15m |
| `GRID_BLACKSTART_DEADLINE` | 黑启动恢复截止 | 20m |

## 主要接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/health` | 健康检查 |
| GET/POST | `/api/v1/battery-cabins` | 列出/注册电池舱 |
| PUT | `/api/v1/battery-cabins/{id}/telemetry` | 更新遥测（温度/SOC） |
| GET | `/api/v1/loads` | 列出负荷 |
| POST | `/api/v1/inspections` | 创建巡检任务 |
| POST | `/api/v1/inspections/{id}/result` | 记录巡检结果 |
| POST | `/api/v1/anomalies` | 异常上报 |
| GET | `/api/v1/workorders` | 列出工单 |
| POST | `/api/v1/workorders/{id}/accept` | 接单 |
| POST | `/api/v1/workorders/{id}/start` | 开始维修 |
| POST | `/api/v1/workorders/{id}/worsen` | 异常恶化（追加关联工单） |
| POST | `/api/v1/workorders/{id}/complete` | 完成工单 |
| POST | `/api/v1/blackstarts` | 发起黑启动 |
| POST | `/api/v1/blackstarts/{id}/start` | 启动黑启动 |
| POST | `/api/v1/blackstarts/{id}/restore-bus` | 恢复母线电压 |
| POST | `/api/v1/blackstarts/{id}/network-interrupt` | 上报网络中断 |
| POST | `/api/v1/blackstarts/{id}/restore-network` | 恢复网络 |
| POST | `/api/v1/blackstarts/{id}/complete` | 完成黑启动 |
| POST | `/api/v1/grid-connections` | 申请并网 |
| POST | `/api/v1/grid-connections/{id}/sign` | 复核签字 |
| POST | `/api/v1/grid-connections/{id}/synchronize` | 自动同期并网 |
| GET | `/api/v1/sms` | 列出短信指令 |

## 测试

```bash
go test -timeout=120s -count=1 ./...
```

测试覆盖正常路径、错误路径、状态迁移、并发竞争、超时升级、截止失败、网络中断短信重发等场景，不依赖任何外部服务。

## 技术栈

- Go 1.26，仅使用标准库（net/http、encoding/json、sync），无外部依赖
- 线程安全内存存储，单互斥锁保证复合操作原子性
- 后台调度器基于 context 取消，支持优雅停机
