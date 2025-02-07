## JH-OSS

精弘对象存储服务

### 代码格式检查

```shell
gofmt -w .
gci write . -s standard -s default
golangci-lint run --config .golangci.yml
```
