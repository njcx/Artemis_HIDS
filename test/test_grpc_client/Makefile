.PHONY: help build run clean test multiple stop

help: ## 显示帮助信息
	@echo "HIDS Test gRPC Client - Makefile"
	@echo ""
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 构建测试客户端
	@./build.sh

run: ## 运行单个测试客户端 (默认参数)
	@./test-client

fast: ## 快速测试 (1秒间隔, 5个事件)
	@./test-client -batch 1 -batch-size 5

stress: ## 压力测试 (1秒间隔, 100个事件)
	@./test-client -batch 1 -batch-size 100

file-only: ## 只测试文件监控事件
	@./test-client -process=false -network=false

process-only: ## 只测试进程监控事件
	@./test-client -file=false -network=false

network-only: ## 只测试网络监控事件
	@./test-client -file=false -process=false

multiple: ## 启动多个客户端 (默认5个)
	@./run_multiple.sh $(n)

multiple-10: ## 启动10个客户端
	@./run_multiple.sh 10

multiple-20: ## 启动20个客户端
	@./run_multiple.sh 20

stop: ## 停止所有测试客户端
	@pkill -f test-client || true
	@echo "All test clients stopped"

logs: ## 查看第一个客户端的日志
	@tail -f logs/client-1.log

clean: ## 清理构建文件和日志
	@rm -f test-client
	@rm -rf logs/
	@echo "Cleaned"

test: build run ## 构建并运行

examples: ## 显示使用示例
	@./examples.sh

.DEFAULT_GOAL := help
