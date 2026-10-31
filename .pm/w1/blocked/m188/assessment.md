# 旧版运行时核查 — 2026-10-29（UTC 2026-10-30）

最新上游检查：00:56:15Z；生产检查：00:59:28Z。checkout `e3d19da31`。11 个官方 tag 均与 2026-10-28 记录的不可变 digest 相同，尚未包含系统修复。没有因为缺 SBOM 将任何版本算作安全。

| 运行时 | 系统 libbrotli1 | 运行时自带 Brotli | 当前运行服务数 | 处置 |
| --- | --- | --- | --- | --- |
| Node 14.21.3 | 1.0.9-2+b2 | 1.0.9 | 0 | Bullseye 同源回移系统补丁；原 Node 版本重编译匹配的 Brotli 1.0.10 |
| Node 16.20.2 | 1.0.9-2+b6 | 1.0.9 | 0 | Bookworm 安全包；原 Node 版本重编译匹配的 Brotli 1.0.10 |
| Node 18.20.8 | 1.0.9-2+b6 | 1.1.0 | 0 | Bookworm 安全包；原 Node 版本重编译匹配的 Brotli 1.1.1 |
| Node 20.20.2 | 1.0.9-2+b6 | 1.1.0 | 0 | Bookworm 安全包；原 Node 版本重编译匹配的 Brotli 1.1.1 |
| Python 3.7.17 | 1.0.9-2+b2 | 标准解释器未发现自带库 | 0 | Bullseye 同源回移系统补丁 |
| Python 3.8.20 | 1.0.9-2+b6 | 标准解释器未发现自带库 | 0 | Bookworm 安全包 |
| Python 3.9.25 | 1.0.9-2+b6 | 标准解释器未发现自带库 | 0 | Bookworm 安全包 |
| Python 3.10.22 | 1.0.9-2+b6 | 标准解释器未发现自带库 | 0 | Bookworm 安全包 |
| Ruby 3.0.7 | 1.0.9-2+b2 | 标准解释器未发现自带库 | 0 | Bullseye 同源回移系统补丁 |
| Ruby 3.1.7 | 1.0.9-2+b6 | 标准解释器未发现自带库 | 0 | Bookworm 安全包 |
| Ruby 3.2.11 | 1.0.9-2+b6 | 标准解释器未发现自带库 | 0 | Bookworm 安全包 |

## 证据与限制

- [本次 tag 解析](evidence/upstream-digests.json) 与 [原不可变 SBOM 记录](../../evidence/136-brotli-upstream-20261028.json) 共同支撑已有 8 个包记录；Node14、Node16、Python3.7 的缺失项由固定 digest 的 arm64 容器直接检查：[Node14](evidence/node14-probe.json)、[Node16/Python3.7](evidence/missing-sbom.json)。这不是全部架构发布验收，发布时仍必须双架构检测。
- [CPython/Ruby 精确版本源码树](evidence/runtime-source-trees.json) 均未截断且没有 Brotli 路径；Python3.7 实测没有预装 Python Brotli 模块。结论只涉及基础解释器，不承诺用户安装的 pip/gem/npm 依赖已修复。
- [生产只读记录](evidence/production-usage.json)：30 个 App；两个运行中的 native Node 服务分别是 24.22.1 / 22.23.4，系统库仍为 `1.0.9-2+b6`，由独立的 136 重建。另两个 Node App 失败或暂停且没有 pod，不能宣称其将来的版本已核实；没有 native Python/Ruby 或 CNB App。
- 生产 SSH 访问通过成功的 [app-cluster run 40979246716](https://github.com/bex-co/bex/actions/runs/40979246716) 中受信任的稳定 CP 指纹与当前节点一致后恢复，未放宽 host-key 验证；凭据不进入 board。
- [Debian 安全记录](https://security-tracker.debian.org/tracker/CVE-2026-111497) 明确 Bookworm 修复包及 Google 的 1.0.10/1.1.1 回移。固定来源的 Debian Bookworm 修复源码将在 Bullseye 原 libc 上编译，不把 Bookworm 二进制直接塞进 Bullseye。
- 本项修复的是 11 个已审核 line pin；用户请求其他精确 patch 时仍按既有 Render 合同解析官方镜像，不能把这些未独立修补的任意精确版本称为已修复，也不能静默替换用户版本。

## 顺序与估算

目前无在用旧版，因此按表顺序覆盖所有 11 个版本，先验证最旧的 Node14 和 Bullseye 系统回移，再验证 Node20 / Bookworm，对照现代安全版本。没有删除、禁用或升级旧版支持。

复用 main 已有 `deploy/native-base/` 与 `native-base-image.yml`，不新增平行发布框架。t002 的 2h 是工程工作初估；四个完整 Node 版本的双架构重编译加发布测试可能需要数小时，不能把签名发布、接入与线上验收算作已经完成。t003/t004 仍须使用实际发布的 digest 和真实 rollout 时间。
