# BENZHI_README

## 项目说明
- 项目：benzhi-project-a7d1a41f-f002-4f6e-b8dc-f5dbb757b706
- 项目用途：已完整实现植物引种隔离放行工作台：Go 单进程服务提供浏览器工作台与同源 JSON 接口，以 SQLite 事务承载建档、风险基线、专业审查、连续观察、偏差限制与验证、资格核验、最终放行或终止归档，并支持 revision 乐观锁、request_id 幂等和只追加审计时间线。
- Go 工具链：`golang:1.23.0`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：植物引种隔离放行工作台
- 项目介绍：面向植物园引种与生物安全团队的单流程 Web 应用，用于把外来植物材料从隔离建档、风险审查、连续观察、偏差处置推进到放行或终止归档。项目按 standard 档位规划，目标不少于 2000 行真实生产 Go 代码和 20 个生产 Go 文件，不包含测试与前端资源。
- 项目概述：面向植物园引种与生物安全团队的单流程 Web 应用，用于把外来植物材料从隔离建档、风险审查、连续观察、偏差处置推进到放行或终止归档。项目按 standard 档位规划，目标不少于 2000 行真实生产 Go 代码和 20 个生产 Go 文件，不包含测试与前端资源。
- 核心工作流：引种管理员创建隔离个案并提交风险资料，审核员确认隔离方案后启动观察；观察人员持续登记植株状态，发现偏差时完成限制措施与复核，满足观察期限和证据完整性后由审核员作出放行或终止决定并归档。
- 对外接口：Go 服务在同一高位回环地址提供原生 HTML、CSS、JavaScript 浏览器工作台和同源 JSON 接口；页面包含待办列表、个案状态时间线、风险审查表、观察记录、偏差处置和最终决定面板，无需 Node 构建链即可完成全部主流程。服务支持 -addr=127.0.0.1:<port>，默认监听 127.0.0.1:19081，禁止默认绑定 8080、80、3000 或 0.0.0.0；自检也必须使用显式传入的回环地址。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-a7d1a41f-f002-4f6e-b8dc-f5dbb757b706-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-a7d1a41f-f002-4f6e-b8dc-f5dbb757b706-arm64 linux/arm64

docker run -it benzhi-project-a7d1a41f-f002-4f6e-b8dc-f5dbb757b706-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19081`
