## Cube-Go

精弘存储立方后端仓库

### 构建

依赖 `libwebp`

```shell
# Ubuntu/Debian
sudo apt install libwebp-dev

# macOS
brew install webp
```

Windows 用户需自行配置 MinGW 环境

为了方便部署，您可以手动触发 `Build` 工作流来构建全平台的二进制文件

### 代码格式检查

需要安装 [golangci-lint](https://golangci-lint.run/)

```shell
gofmt -w .
goimports -w .
golangci-lint run --config .golangci.yml
```
