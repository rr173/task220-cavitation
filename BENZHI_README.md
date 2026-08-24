基于 Go 实现的船舶螺旋桨空化声纹判定服务，一款纯后端水动力分析服务，处理多通道声纹、空化判定与诊断结果发布。

# BENZHI 评测说明

本文件供评测构建与验证使用，说明 `--smoke-test` 契约、Docker 双架构
构建与 API 入口。

## 构建命令

```bash
# 本地构建 / 检查 / 测试
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...

# 端到端冒烟（唯一判据：exit 0）
go run ./cmd/cavitation --smoke-test
```

## --smoke-test 契约

`--smoke-test` 不启动长驻服务，而是：

1. 打开 SQLite 数据库 A；
2. 创建试验 -> 开始采集 -> 多通道接收 20 窗口声纹 -> 幂等去重验证
   -> 通道延迟校准 -> 标记机械噪声通道 -> 结束采集 -> 分析并检测空化
   事件 -> 确认试验 -> 发布结论包（封存）-> 封存后拒绝写入验证；
3. 关闭数据库 A，重开同一路径数据库 B，验证数据与状态仍在；
4. 以退出码 0 结束（任何断言失败退出码 1）。

## Docker 双架构构建

```bash
# 构建镜像（默认平台 linux/amd64，可传 linux/arm64）
bash build_benzhi_docker.sh my-project linux/amd64
bash build_benzhi_docker.sh my-project linux/arm64

# 运行冒烟
docker run --rm my-project:latest --smoke-test

# 启动服务
docker run --rm -p 8080:8080 my-project:latest --addr :8080
```

镜像内已设置 `ENTRYPOINT ["/app/cavitation"]` 与 `CMD ["--smoke-test"]`，
直接 `docker run --rm <image>` 即执行冒烟，无需追加路径参数。

## API 入口（统一 `/api` 前缀）

- 试验：`POST /api/trials`、`GET /api/trials`、`GET /api/trials/{id}`、
  `POST /api/trials/{id}/start|finish|confirm`
- 片段：`POST /api/trials/{id}/segments`、`GET /api/trials/{id}/segments`、
  `POST /api/segments/{id}/noisy`、`POST /api/trials/{id}/calibrate`、
  `GET /api/trials/{id}/delays`
- 分析/事件：`POST /api/trials/{id}/analyze`、`GET /api/trials/{id}/events`、
  `GET /api/events/{id}`、`POST /api/events/{id}/reject|advance`、
  `GET /api/trials/{id}/features`
- 结论/阈值：`POST /api/trials/{id}/packages`、`GET /api/trials/{id}/packages`、
  `GET /api/packages/{id}`、`GET /api/thresholds`、`POST /api/thresholds`
- 统计/健康：`GET /api/stats`、`GET /api/health`

## 环境

- Go 1.26.3、`GOTOOLCHAIN=local`、`CGO_ENABLED=0`
- SQLite 3.46.1（`modernc.org/sqlite v1.52.0`，纯 Go 驱动）
