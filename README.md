## Cube-Go

精弘对象存储中间件服务端

### 构建

由于调用了 `libwebp`，需要启用 `CGO` 并安装 [GCC](http://tdm-gcc.tdragon.net/download)

为了方便部署，您可以手动触发 `Build` 工作流来构建全平台的二进制文件

### 代码格式检查

需要安装 [gci](https://github.com/daixiang0/gci) 和 [golangci-lint](https://golangci-lint.run/)

```shell
gofmt -w .
gci write . -s standard -s default
golangci-lint run --config .golangci.yml
```
