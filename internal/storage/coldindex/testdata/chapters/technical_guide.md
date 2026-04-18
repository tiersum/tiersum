# Kubernetes 架构指南

本文介绍 Kubernetes 的核心架构组件。

## 控制平面

控制平面是 Kubernetes 的大脑，负责管理集群状态。

### API Server

API Server 是控制平面的前端，处理所有 REST 请求。

主要功能包括：

1. **认证**：验证用户身份
2. **鉴权**：检查操作权限
3. **准入控制**：验证资源规范

### etcd

etcd 是 Kubernetes 的分布式键值存储，保存所有集群数据。

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example
spec:
  containers:
  - name: nginx
    image: nginx:latest
```

### Scheduler

Scheduler 负责将 Pod 调度到合适的节点。

调度流程：

1. 监听未调度的 Pod
2. 筛选可用节点
3. 为节点打分
4. 选择最优节点
5. 绑定 Pod 到节点

## 工作节点

工作节点运行实际的应用容器。

### Kubelet

Kubelet 是运行在每个节点上的代理，负责：

- 接收 API Server 的指令
- 管理 Pod 生命周期
- 报告节点状态

### 容器运行时

支持多种容器运行时：

- Docker
- containerd
- CRI-O

### Kube-proxy

Kube-proxy 负责 Service 的网络代理和负载均衡。

```bash
# 查看 Service 规则
iptables -t nat -L KUBE-SERVICES
```
