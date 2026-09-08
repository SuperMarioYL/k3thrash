[English](./README.en.md) · [Website](https://k3thrash.lei6393.com) · [GitHub](https://github.com/SuperMarioYL/k3thrash)

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/hero-dark.svg">
  <img src="./assets/presentation/hero-light.svg" width="960" alt="Hero diagram">
</picture>

# k3thrash

**让保存的 I/O 轨迹更容易解读。**

k3thrash 采集 Linux 进程读取计数，结合可用 token 计数，输出相对拓扑基线的读取分类；保存的轨迹报告也可在 macOS 生成。

## 为什么需要它

仅看吞吐无法知道每个 token 伴随多少存储流量。把累计字节与 token 采样保存在同一轨迹，可得到可检查的比值与可分享的报告。

- **保留采样** — 轨迹 JSON 同时保存计数器与拓扑。
- **解释比值** — 报告展示基线与每 token 字节数。
- **离线生成报告** — 离开 Linux 节点也能检查保存轨迹。

## 架构

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/architecture-dark.svg">
  <img src="./assets/presentation/architecture-light.svg" width="960" alt="Architecture diagram">
</picture>

procio 采样 /proc/PID/io，tokensource 读取支持的 llama.cpp 时序行；trace 计算差值，thrash 将每 token 字节与活跃专家数乘以专家打包字节的基线比较，render 输出裁决与趋势线。内置 kimi-k3 常数是当前实现采用的假设。

| 组件 | 职责 |
| --- | --- |
| `Process counters` | internal/procio |
| `Token timing` | internal/tokensource |
| `Trace deltas` | internal/trace |
| `Baseline verdict` | internal/thrash; internal/topo |
| `ASCII report` | internal/render |

## 安装与快速上手

使用仓库清单声明的运行时版本。以下源码安装步骤可复现随仓示例。

```bash
git clone https://github.com/SuperMarioYL/k3thrash.git
cd k3thrash
go build ./cmd/k3thrash
```

需要 Go 1.24+ 与 Python 3；无需运行模型即可渲染随仓六点轨迹。

```bash
python3 examples/presentation_demo.py
```

## 实际运行示例

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/process-dark.svg">
  <img src="./assets/presentation/process-light.svg" width="960" alt="Process diagram">
</picture>

The synthetic trace is classified as pathological thrash at 3.0x its configured baseline.

```text
=== k3thrash report ===
model: kimi-k3   pid: 4242   topo: kimi-k3 (896 experts / 16 active)
window: 13:45:00 → 13:45:25   samples: 6

verdict:
  re-read rate 3.0× reuse: pathological thrash
  re-read rate 3.00× reuse | reuse ratio 0.33 | classification: pathological thrash
nvme read 77309411328.00 B/token (pathological baseline 25769797776.00 B/token)

re-read-rate sparkline (bytes/token, min-max normalized):
  ▅▅▅▅▅
  min 77309411328 | max 77309411328 B/token across 5 samples

share: paste the block above into your thread / issue.
=== end k3thrash report ===
```

完整命令与输出保存在 [docs/demo-results.json](./docs/demo-results.json). 输入和复现代码均随仓提供。

![已有 fixture 报告录制](./assets/demo/k3thrash-demo.gif)

## 用法

安装后在仓库根目录运行以下命令；处理自己的数据时替换相应路径。

```bash
go run ./cmd/k3thrash report examples/trace.example.json
# Linux only; replace the PID and timing file for your process:
go run ./cmd/k3thrash attach --pid 1234 --expert-topo kimi-k3 --token-source decode.log --interval-ms 100 --out trace.json
go run ./cmd/k3thrash report trace.json
```

## 配置

attach 提供 --pid、--token-source（文件/FIFO 或 - 标准输入）、--out、--interval-ms 与 --no-verdict。采样间隔应为正数，默认 100ms。内置注册表支持 kimi-k3 及其别名。比值 <1 为 healthy_warmup，1 到 <2 为 partial_warm，>=2 为 pathological_thrash；这些是诊断分类，不是独立标定的专家驻留测量。

## 集成与职责分工

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/integrations-dark.svg">
  <img src="./assets/presentation/integrations-light.svg" width="960" alt="Integrations diagram">
</picture>

根据工作流选择输入与输出路径。本文本地示例验证其中明确说明的子流程。

| 路径 | 已实现职责 |
| --- | --- |
| Linux /proc | Process read_bytes counter |
| Timing file / stdin | Supported token count lines |
| Trace JSON | Samples and topology fields |
| Terminal report | Verdict and sparkline |

## 限制与后续方向

- 示例输入是合成轨迹，模型和硬件标签属于 fixture 元数据；此处没有吞吐或硬件实测。
- 进程聚合 read_bytes 无法识别具体 NVMe 设备、专家或缓存未命中的因果；分类依赖内置拓扑常数。
- 实时 attach 需要 Linux /proc 访问权限。没有递增 token 计数时，每 token 比值不能视作可靠实测；工具用于诊断，不会预取权重。

逐专家插桩、更多拓扑定义与跨节点比较仍是后续方向。

## 许可与贡献

许可见 [LICENSE](./LICENSE). 反馈问题时请提供最小输入、执行命令和实际输出。
