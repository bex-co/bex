# m188 · 公共底集成交接

11/11 旧运行时补丁产物已完成签名发布和独立验收。生产接入与线上修复尚未完成：用户已选定单一 Bookworm 公共底，逐语言镜像现在只作为候选运行时 donor 与补丁证据。

当前需要 **worker8 完成 [w8/m91/t007](../../../w8/m91/t007.md)**：给出已审核、固定 digest 的公共底，以及同版本旧运行时在其上的真实双架构生成镜像资格。系统库、运行时自带 Brotli、私有 Node、库/头文件/扩展、Poetry 和 Node20 companion 均需各自证明；保留所有原始扫描发现。复制旧 `/usr/local` 不代表发行版或运行时闭包已经兼容。

无需重新选择 A/C、交付手动证书或新建资源。需要调度时，可先优先推进 `$loopx w8` 的 m91/t007；实际资格证据可用后，`$loopx w1` 继续联合接入、六项真实构建/HTTP、签名部署 Ready、新鲜全量服务清单与按需精确重建。

| 运行时 | 已验证的精确版本（候选 donor） |
| --- | --- |
| Node | 14.21.3, 16.20.2, 18.20.8, 20.20.2 |
| Python | 3.7.17, 3.8.20, 3.9.25, 3.10.22 |
| Ruby | 3.0.7, 3.1.7, 3.2.11 |

[完整交接 JSON](evidence/common-integration-handoff/common-integration-handoff.json)包含每个固定引用、producer/verifier、证明哈希、实际扫描处置、公共底资格条件和后续运行步骤；[原中文说明](evidence/common-integration-handoff/handoff.zh.txt)保留详细边界。

本项不会临时写入逐语言租户基础镜像。旧 collector-v2 和直接11-pin写入计划已退役；后续应按实际合格公共底与运行时模型重新审阅采集工具。旧清单中的五行未知对象及所有新 App 必须在真实新 operator Ready 后重新核对，旧报告、无 Pod 或发布数量都不能充当最终安全或线上验收。
