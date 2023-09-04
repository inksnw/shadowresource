[文档参考](http://inksnw.asuscomm.com:3001/post/%E7%94%A8%E8%81%9A%E5%90%88api%E5%AE%9E%E7%8E%B0%E6%8A%BD%E8%B1%A1%E8%B5%84%E6%BA%90/)

## 功能实现

- [x] 创建批量提交
- [x] 删除
- [x] 列表带透传
- [x] 详情/编辑带透传
- [ ] restmapper自动更新

## 本地运行

### 注册aa

> 注意服务使用了externalName模式, 请保证配置的域名能解析到你的开发机

```bash
# 注册aa
kubectl apply -f deploy/api.yaml
# 提交crd
kubectl apply -f deploy/crd.yaml
```

查看注册状态

```bash
➜ kubectl get apiservice v1.apis.abc.com
NAME              SERVICE         AVAILABLE   AGE
v1.apis.abc.com   default/myapi   True        41s
```

### 启动测试

```bash
go run cmd/main.go
```

测试yaml为根目录的`1.yaml`

```bash
kubectl apply -f 1.yaml
kubectl get shadowresource
kubectl get shadowresource task1
kubectl get shadowresource task1 -o yaml
```

## 开发指南

```bash
openapi-gen --input-dirs "k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/version"  --input-dirs github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1   -p github.com/inksnw/shadowresource/pkg/apis/shadowresource/v1 -O zz_generated.openapi --go-header-file=/Users/inksnw/go/src/github.com/inksnw/shadowresource/hack/boilerplate.go.txt
```

