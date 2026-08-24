# task220-cavitation 船舶螺旋桨空化声纹判定服务

面向船舶水动力工程师的后端服务：从多通道螺旋桨水下声纹中判定空化的
起始、持续与消退时刻，并保留声纹证据与判定结论。核心是"谐波缺口比"——
空化起始时空化泡噪声填充螺旋桨谐波谱线之间的频隙，宽带能量相对谐波
能量上升，据此跨窗口追踪空化事件生命周期。

## 业务闭环

1. 登记水池试验（转速、来流压力、参考通道）；
2. 开始采集，多通道接收声纹片段（幂等去重、采样率一致、时间单调）；
3. 多通道延迟校准（互相关对齐）；
4. 标记机械噪声通道（不删除原片段，仅降权）；
5. 结束采集进入分析，逐窗提取谐波特征并检测空化事件；
6. 确认试验，发布带置信度的结论包并封存。

## 标准命令

```bash
# 构建 / 静态检查 / 测试
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...

# 端到端冒烟（真实创建数据、关闭并重开数据库验证持久化与重启恢复）
go run ./cmd/cavitation --smoke-test

# 启动服务
go run ./cmd/cavitation --addr :8080 --db ./cavitation.db
```

## API 入口（统一 `/api` 前缀）

| 能力 | API | 说明 |
| --- | --- | --- |
| 登记试验 | `POST /api/trials` | 转速/压力/参考通道 |
| 试验列表 | `GET /api/trials` | 全部试验 |
| 试验详情 | `GET /api/trials/{id}` | 单个试验 |
| 开始采集 | `POST /api/trials/{id}/start` | preparing -> acquiring |
| 结束采集 | `POST /api/trials/{id}/finish` | acquiring -> analyzing |
| 确认试验 | `POST /api/trials/{id}/confirm` | analyzing -> confirmed |
| 接收片段 | `POST /api/trials/{id}/segments` | 多通道声纹抽样 |
| 片段列表 | `GET /api/trials/{id}/segments` | 试验全部片段 |
| 标记噪声 | `POST /api/segments/{id}/noisy` | 机械噪声降权 |
| 通道校准 | `POST /api/trials/{id}/calibrate` | 互相关延迟对齐 |
| 通道延迟 | `GET /api/trials/{id}/delays` | 校准结果 |
| 分析 | `POST /api/trials/{id}/analyze` | 特征提取 + 事件检测 |
| 事件列表 | `GET /api/trials/{id}/events` | 空化事件 |
| 事件详情 | `GET /api/events/{id}` | 单个事件 |
| 否决事件 | `POST /api/events/{id}/reject` | 标注误报原因 |
| 推进阶段 | `POST /api/events/{id}/advance` | 手动推进状态机 |
| 特征列表 | `GET /api/trials/{id}/features` | 谐波特征 |
| 发布结论 | `POST /api/trials/{id}/packages` | 冻结结论并封存 |
| 结论列表 | `GET /api/trials/{id}/packages` | 结论包 |
| 结论详情 | `GET /api/packages/{id}` | 单个结论包 |
| 阈值列表 | `GET /api/thresholds` | 判定阈值版本 |
| 新增阈值 | `POST /api/thresholds` | 新阈值版本 |
| 统计 | `GET /api/stats` | 全局统计 |
| 健康 | `GET /api/health` | 存活探针 |

## 持久化

SQLite（`modernc.org/sqlite`，纯 Go 驱动，CGO 无关，离线可构建）。表：
`trials`、`segments`、`channel_delays`、`harmonic_features`、`events`、
`packages`、`thresholds`。片段按 `(trial_id, channel_index, start_time_ms)`
幂等去重；封存试验不可再写入；发布结论包冻结事件与阈值版本快照。

## 技术栈

- Go 1.26.3（`GOTOOLCHAIN=local`，`CGO_ENABLED=0`）
- SQLite 3.46.1（`modernc.org/sqlite v1.52.0`）
- 纯标准库 HTTP（`net/http`，Go 1.22+ 路由模式）
