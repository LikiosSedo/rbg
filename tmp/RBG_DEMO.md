# KEP-8 RoleTemplate 功能演示

## 一、背景与动机

### 1.1 问题场景

> 管理多角色 RBG 部署需要在每个 role 中重复基础配置：
> - 每个 role 重复 image、volumes、resources、环境变量
> - 更新共享配置需要修改每个 role
> - 手动更新容易导致配置漂移和不一致

### 1.2 SGLang PD 分离部署

以 `examples/pd-disagg/sglang/sglang-pd.yaml` 为例，prefill 和 decode 两个 role 重复了大量配置：

| 重复字段 | prefill | decode | 完全相同 |
|---------|---------|--------|----------|
| image | sglang:v0.5.3 | sglang:v0.5.3 | 是 |
| volumes (model) | llm-model PVC | llm-model PVC | 是 |
| volumes (shm) | emptyDir 64Gi | emptyDir 64Gi | 是 |
| volumeMounts | /models, /dev/shm | /models, /dev/shm | 是 |
| resources | 2 GPU, rdma/hca | 2 GPU, rdma/hca | 是 |
| readinessProbe | tcpSocket:8000 | tcpSocket:8000 | 是 |
| env: POD_IP | fieldRef | fieldRef | 是 |
| **command** | --disaggregation-mode prefill | --disaggregation-mode decode | 否 |

**问题**：大量的配置重复，升级 image 版本需要改两处，容易遗漏导致配置漂移。

### 1.3 KEP-8 解决方案

```yaml
# 定义一次共享配置
roleTemplates:
- name: sglang-base
  template:
    spec:
      volumes: [...]      # 定义一次
      containers:
      - name: sglang
        image: sglang:v0.5.3  # 定义一次
        resources: {...}       # 定义一次

# 各 role 引用 + 覆盖差异
roles:
- name: prefill
  templateRef: { name: sglang-base }
  templatePatch:
    spec:
      containers:
      - name: sglang
        command: [...--disaggregation-mode prefill...]  # 仅覆盖差异

- name: decode
  templateRef: { name: sglang-base }
  templatePatch:
    spec:
      containers:
      - name: sglang
        command: [...--disaggregation-mode decode...]   # 仅覆盖差异
```

---

## 二、Demo 演示

> **说明**：本 demo 使用 env 变量简化演示，实际生产场景中 KEP-8 主要用于减少 image、volumes、resources、volumeMounts 等重配置的重复。

### 2.1 Demo YAML 结构

```yaml
apiVersion: workloads.x-k8s.io/v1alpha1
kind: RoleBasedGroup
metadata:
  name: roletemplate-demo
spec:
  roleTemplates:
    - name: base-template
      template:
        metadata:
          labels:
            app: demo
        spec:
          containers:
            - name: main
              image: busybox:latest
              command: ["sleep", "infinity"]
              env:
                - name: APP_VERSION
                  value: "v1.0.0"
                - name: SHARED_CONFIG
                  value: "from-roletemplate"
              resources:
                requests:
                  memory: "32Mi"
                  cpu: "50m"

  roles:
    - name: prefill
      replicas: 1
      workload:
        apiVersion: apps/v1
        kind: StatefulSet
      templateRef:
        name: base-template
      templatePatch:
        spec:
          containers:
            - name: main
              env:
                - name: ROLE_NAME
                  value: "prefill"
                - name: BATCH_SIZE
                  value: "512"

    - name: decode
      replicas: 1
      templateRef:
        name: base-template
      templatePatch:
        spec:
          containers:
            - name: main
              env:
                - name: ROLE_NAME
                  value: "decode"
                - name: MAX_TOKENS
                  value: "4096"
```

### 2.2 验证命令与结果

```bash
# 应用资源
kubectl apply -f rbg-roletemplate-demo.yaml

# 查看 Pod 状态
kubectl get pods -l app=demo
```

**输出：**
```
NAME                          READY   STATUS    AGE
roletemplate-demo-prefill-0   1/1     Running   3s
roletemplate-demo-decode-0    1/1     Running   2s
```

**验证共享配置（两个 Pod 相同）：**
```bash
kubectl exec roletemplate-demo-prefill-0 -- env | grep -E "APP_VERSION|SHARED"
kubectl exec roletemplate-demo-decode-0 -- env | grep -E "APP_VERSION|SHARED"
```
```
APP_VERSION=v1.0.0
SHARED_CONFIG=from-roletemplate
```

**验证独有配置（两个 Pod 不同）：**
```bash
# prefill
kubectl exec roletemplate-demo-prefill-0 -- env | grep -E "ROLE_NAME|BATCH"
# 输出: ROLE_NAME=prefill, BATCH_SIZE=512

# decode
kubectl exec roletemplate-demo-decode-0 -- env | grep -E "ROLE_NAME|MAX"
# 输出: ROLE_NAME=decode, MAX_TOKENS=4096
```

### 2.3 合并效果对照表

| 字段 | prefill | decode | 来源 |
|------|---------|--------|------|
| image | busybox:latest | busybox:latest | roleTemplate |
| APP_VERSION | v1.0.0 | v1.0.0 | roleTemplate |
| SHARED_CONFIG | from-roletemplate | from-roletemplate | roleTemplate |
| resources | 32Mi/50m | 32Mi/50m | roleTemplate |
| ROLE_NAME | prefill | decode | templatePatch |
| 独有配置 | BATCH_SIZE=512 | MAX_TOKENS=4096 | templatePatch |

---

## 三、修改 roleTemplate 触发全局 Rollout

### 3.1 操作步骤

```bash
# 修改 roleTemplate 中的 APP_VERSION: v1.0.0 -> v2.0.0
kubectl apply -f rbg-roletemplate-demo.yaml

# 观察 Pod 滚动更新
kubectl get pods -l app=demo -w
```

### 3.2 测试结果

```
NAME                          READY   STATUS        AGE
roletemplate-demo-prefill-0   1/1     Terminating   5m
roletemplate-demo-decode-0    1/1     Terminating   5m
roletemplate-demo-prefill-0   0/1     Pending       0s
roletemplate-demo-decode-0    0/1     Pending       0s
roletemplate-demo-prefill-0   1/1     Running       2s
roletemplate-demo-decode-0    1/1     Running       2s
```

**验证更新生效：**
```bash
kubectl exec roletemplate-demo-prefill-0 -- env | grep APP_VERSION
kubectl exec roletemplate-demo-decode-0 -- env | grep APP_VERSION
```
```
APP_VERSION=v2.0.0  # 两个 Pod 都更新
APP_VERSION=v2.0.0
```

对应 KEP-8 设计文档的 User Story 2：
> SRE 更新 `roleTemplates[0].template.spec.containers[0].image`，controller 自动滚动更新所有引用该模板的 role。

---

## 四、只改 templatePatch 触发单 Role 更新

### 4.1 操作步骤

```bash
# 只修改 prefill 的 templatePatch: BATCH_SIZE: 512 -> 1024
kubectl apply -f rbg-roletemplate-demo.yaml

# 观察 Pod 更新
kubectl get pods -l app=demo -w
```

### 4.2 测试结果

```
NAME                          READY   STATUS        AGE
roletemplate-demo-prefill-0   1/1     Terminating   2m   # 只有 prefill 更新
roletemplate-demo-decode-0    1/1     Running       2m   # decode 不变
roletemplate-demo-prefill-0   1/1     Running       2s
```

**验证：**
```bash
kubectl exec roletemplate-demo-prefill-0 -- env | grep BATCH_SIZE
# 输出: BATCH_SIZE=1024（已更新）

kubectl exec roletemplate-demo-decode-0 -- env | grep MAX_TOKENS
# 输出: MAX_TOKENS=4096（未变）
```

---

## 五、Strategic Merge Patch 合并行为


| 字段 | 合并策略 | 示例 |
|------|---------|------|
| volumes | 按 `name` 合并 | Template 有 "model"，Patch 加 "cache" -> 两者都存在 |
| env | 按 `name` 合并 | Template 有 "POD_IP"，Patch 加 "ROLE" -> 两者都存在 |
| resources | 整体替换 | Template 2 GPU，Patch 4 GPU -> 使用 4 GPU |
| command | 整体替换 | Template 有基础命令，Patch 指定完整命令 -> 使用 Patch |

> 使用 Kubernetes 原生 Strategic Merge Patch 语义。

---

## 六、LWS 三层合并示例

使用 LeaderWorkerSet的场景：

### 6.1 三层合并概念

```
roleTemplate.template       (第一层：共享基础配置)
        ↓
role.templatePatch          (第二层：role 级覆盖)
        ↓
patchLeaderTemplate         (第三层：leader 专属配置)
patchWorkerTemplate         (第三层：worker 专属配置)
```

### 6.2 示例 YAML

```yaml
apiVersion: workloads.x-k8s.io/v1alpha1
kind: RoleBasedGroup
metadata:
  name: lws-demo
spec:
  roleTemplates:
    - name: inference-base
      template:
        spec:
          containers:
            - name: app
              image: sglang:v0.5.3
              resources:
                requests:
                  cpu: "100m"
                  memory: "128Mi"

  roles:
    - name: inference
      replicas: 2
      workload:
        apiVersion: leaderworkerset.x-k8s.io/v1
        kind: LeaderWorkerSet
      templateRef:
        name: inference-base
      templatePatch:                    # 第二层：role 级覆盖
        spec:
          containers:
            - name: app
              resources:
                requests:
                  memory: "256Mi"       # 覆盖 memory
      leaderWorkerSet:
        size: 2
        patchLeaderTemplate:            # 第三层：leader 专属
          metadata:
            labels:
              node-type: leader
          spec:
            containers:
              - name: app
                command: ["--role", "leader", "--port", "8000"]
        patchWorkerTemplate:            # 第三层：worker 专属
          metadata:
            labels:
              node-type: worker
          spec:
            containers:
              - name: app
                command: ["--role", "worker"]
```

### 6.3 合并结果

**Leader Pod：**
| 字段 | 值 | 来源 |
|------|-----|------|
| image | sglang:v0.5.3 | roleTemplate |
| cpu | 100m | roleTemplate |
| memory | 256Mi | templatePatch 覆盖 |
| command | --role leader | patchLeaderTemplate |
| label: node-type | leader | patchLeaderTemplate |

**Worker Pod：**
| 字段 | 值 | 来源 |
|------|-----|------|
| image | sglang:v0.5.3 | roleTemplate |
| cpu | 100m | roleTemplate |
| memory | 256Mi | templatePatch 覆盖 |
| command | --role worker | patchWorkerTemplate |
| label: node-type | worker | patchWorkerTemplate |

### 6.4 修改 templatePatch（第二层）

**操作**：修改 `templatePatch` 中的 memory: 256Mi -> 512Mi

**预期结果**：
- Leader 和 Worker Pod 都重建
- 两者的 memory 都变为 512Mi
- command 和 label 保持不变

```
# kubectl get pods -w
NAME                    READY   STATUS        AGE
lws-demo-inference-0    2/2     Terminating   5m   # group-0 整体更新
lws-demo-inference-1    2/2     Terminating   5m   # group-1 整体更新
lws-demo-inference-0    2/2     Running       3s
lws-demo-inference-1    2/2     Running       3s
```

### 6.5 修改 patchLeaderTemplate（第三层）

**操作**：修改 `patchLeaderTemplate` 中的 command: --port 8000 -> --port 9000

**预期结果**：
- Leader 和 Worker Pod 都重建（LWS 整体滚动）
- 只有 Leader 的 command 变化
- Worker 的 command 保持不变

```
# 验证 Leader command 变化
kubectl get lws lws-demo-inference -o yaml | grep -A5 leaderTemplate
# command: ["--role", "leader", "--port", "9000"]

# 验证 Worker command 不变
kubectl get lws lws-demo-inference -o yaml | grep -A5 workerTemplate
# command: ["--role", "worker"]
```

### 6.6 三层修改影响对照

| 修改层级 | 修改内容 | Leader 影响 | Worker 影响 |
|---------|---------|------------|------------|
| roleTemplate | image: v0.5.3 -> v0.6.0 | 更新 | 更新 |
| templatePatch | memory: 256Mi -> 512Mi | 更新 | 更新 |
| patchLeaderTemplate | port: 8000 -> 9000 | 更新 | 不变（但 Pod 会重建） |
| patchWorkerTemplate | 新增 env: WORKER=true | 不变（但 Pod 会重建） | 更新 |

> LWS 的滚动更新是按 group 整体进行的，即使只改 leader/worker 其中一方，整个 group 都会重建。

---

## 七、E2E 测试覆盖情况

### 7.1 已覆盖场景

| 测试用例 | 说明 | 状态 |
|---------|------|------|
| create rbg with roleTemplates | 基本功能：templateRef + templatePatch 合并 | 已覆盖 |
| empty templatePatch | 空 patch 使用 base template 原样 | 已覆盖 |
| update roleTemplate triggers rollout | 修改 roleTemplate 触发全局更新 + ControllerRevision 递增 | 已覆盖 |
| mixed mode | templateRef 和传统 template 共存 | 已覆盖 |
| LWS two-layer patch | LWS 三层合并（roleTemplate -> templatePatch -> leader/workerPatch） | 已覆盖 |
| rollback roleTemplate | 回滚到之前版本，workload spec 恢复 | 已覆盖 |


## 八、API 设计演进

### 8.1 KEP-8 原始设计

KEP-8 原始设计文档中，`template` 和 `templateRef` 作为 `RoleSpec` 的直接字段：

```go
// 原始设计（概念）
type RoleSpec struct {
    Name        string
    Replicas    *int32
    Template    *corev1.PodTemplateSpec  // 直接字段
    TemplateRef *TemplateRef             // 直接字段
    TemplatePatch runtime.RawExtension
    // ...
}
```
对应 YAML：
```yaml
roles:
- name: prefill
  template:           # 直接在 role 下
    spec: {...}
# 或
- name: decode
  templateRef:        # 直接在 role 下
    name: base
  templatePatch:
    spec: {...}
```

### 8.2 后续实现：TemplateSource 联合类型 + inline

实际实现引入了 `TemplateSource` 联合类型，然后通过 `json:",inline"` 内联到 `RoleSpec`：

```go
// TemplateSource 定义联合类型
// +kubebuilder:validation:XValidation:rule="!(has(self.template) && has(self.templateRef))",message="template and templateRef are mutually exclusive"
type TemplateSource struct {
    Template    *corev1.PodTemplateSpec `json:"template,omitempty"`
    TemplateRef *TemplateRef            `json:"templateRef,omitempty"`
}

type RoleSpec struct {
    Name        string
    Replicas    *int32
    // 通过 inline 内联 TemplateSource
    TemplateSource `json:",inline"`
    TemplatePatch  runtime.RawExtension `json:"templatePatch,omitempty"`
    // ...
}
```

### 8.3 为什么要这样改

| 原始设计问题 | inline 方案解决 |
|-------------|----------------|
| `template` 和 `templateRef` 互斥校验难以在 CRD 层实现 | `TemplateSource` 联合类型可定义 XValidation |
| 如果用嵌套结构会改变 YAML 格式 | `json:",inline"` 保持 YAML 结构不变 |
| 直接字段无法复用校验逻辑 | 联合类型封装校验，可复用于其他资源 |

**关键点**：`json:",inline"` 使得 Go 结构体的层级（`RoleSpec -> TemplateSource -> Template`）在 YAML 中被展平为 `role.template`，用户无感知内部实现变化。

### 8.4 校验分层

| 校验规则 | 校验位置 | 原因 |
|---------|---------|------|
| `template` 和 `templateRef` 互斥 | CRD XValidation（TemplateSource） | CEL 可检查标准字段 |
| 非 InstanceSet 必须设置其一 | CRD XValidation（RoleSpec） | CEL 可检查 workload.kind |
| InstanceSet 不支持 templateRef | CRD XValidation（RoleSpec） | CEL 可检查 workload.kind |
| templatePatch 仅在 templateRef 时有效 | Controller 层校验 | `templatePatch` 是 `runtime.RawExtension`（`x-kubernetes-preserve-unknown-fields`），CEL 无法检查 |

### 8.5 XValidation 规则

```go
// TemplateSource 层
// +kubebuilder:validation:XValidation:rule="!(has(self.template) && has(self.templateRef))",message="template and templateRef are mutually exclusive"

// RoleSpec 层
// +kubebuilder:validation:XValidation:rule="!has(self.templateRef) || !has(self.workload) || self.workload.kind != 'InstanceSet'",message="templateRef is not supported for InstanceSet workloads"

// +kubebuilder:validation:XValidation:rule="(has(self.template) != has(self.templateRef)) || (has(self.workload) && self.workload.kind == 'InstanceSet')",message="template or templateRef must be set for non-InstanceSet workloads"
```
---

## 九、关键设计点总结

1. **向后兼容**：使用 `json:",inline"` 保持 YAML 结构不变，`role.template` 仍可直接使用

2. **互斥校验**（CRD XValidation）：
   - `template` 和 `templateRef` 不能同时设置
   - 非 InstanceSet workload 必须二选一

3. **InstanceSet 兼容**：InstanceSet 不支持 templateRef（可使用 components）

4. **LWS 三层合并**：roleTemplate -> templatePatch -> patchLeaderTemplate/patchWorkerTemplate

---
