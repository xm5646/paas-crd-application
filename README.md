# Kubernetes CRD Application
### crd说明
> 定义app对象，对应到实际项目的应用结构，一个app包含多个module,每个Module是一个服务,每个服务对应一个k8s deployment
### 控制器功能
- 自动根据modules信息检查服务运行情况,并更新app状态
- 根据module中的配置,自动创建deployment以及svc
- 根据module中的proxies信息, 自动更新ingress tcp/udp configmap信息

### crd yaml定义示例
```
apiVersion: app.dsgkinfo.com/v1
kind: Application
metadata:
  name: wsgw
  labels:
    app: wsgw
spec:
  userID: 2
  description: wsgw后台接口服务
  modules:
    - name: web
      proxies: #ingress l4 config map 配置信息
        - protocol: tcp
          port: 9098
          targetPort: 80
      template:
        replicas: 0
        template:
          metadata:
            labels:
              name: web
          spec:
            containers:
              - name: nginx
                image: 192.168.31.132/appdeploy/tomcat:7
                imagePullPolicy: IfNotPresent
                ports:
                  - containerPort: 80
            restartPolicy: Always
        selector:
          matchLabels:
            name: web
```
### 构建运行
#### 构建
```
make && make install
```

#### 制作部署镜像
```
make docker-build IMG=192.168.31.136/dashuo_containers/application-controller:v0.8
```

#### 生成yaml
```
kustomize build config/default >controller-deploy.yaml
```
