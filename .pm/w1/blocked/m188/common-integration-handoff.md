# m188 · 公共底集成交接

11/11 旧运行时补丁产物已完成签名发布和独立验收。生产接入与线上修复尚未完成：用户已选定单一 Bookworm 公共底，逐语言镜像现在只作为候选运行时 donor 与补丁证据。

**当前需要用户帮助（2026-11-01）：** 协作代码1feba939b已发布，m188因worker缺少可用的新CI触发入口停放；从可用GitHub终端运行 `gh workflow run native-base-image.yml --repo bex-co/bex --ref main -f runtime=node-14.21.2-bullseye`，或恢复worker现有dispatch入口。实际run可读取后先去重/审阅；D1/D2无需再选。公共底41341287851正在正常执行，不取消也不把等待当成阻塞。详细范围与全部尚未完成的验收见[当前记录](README.md#当前阻塞与用户最小动作--2026-11-01-1938-utc)。

按2026-11-01用户 **D2=A**，当前由 **worker1 与 worker8 协同推进 [w8/m91/t007](../../../w8/blocked/m91/t007.md)**；w1的下一项为t009。已有[只供测试的公共底候选审阅](../../../w8/blocked/m91/evidence/common-artifact-review-20261101/README.md)，仍需补齐同一固定公共底的完整两架构utility baseline和运行时/companion资格，再评审生产接入。系统库、运行时自带 Brotli、私有 Node、库/头文件/扩展、Poetry 和 Node20 companion 均需各自证明；保留所有原始扫描发现。复制旧 `/usr/local` 不代表发行版或运行时闭包已经兼容。

用户已完成分工选择，无需重新选型、交付手动证书或新建资源。下一步 `$loopx w1` 从t009开始：先对接现有候选/活动run，复用共享流水线取得完整utility baseline并协作补齐旧版本资格，避免重复发布。采用所需全部资格齐备后，再继续联合接入、六项真实构建/HTTP、签名部署Ready、新鲜全量服务清单与按需精确重建；不单凭11份donor或同意协作就采用。

2026-10-31 合并 `37ec75d71` 后，精确旧版本的公共底源码准备已有进展，完整 build 包测试通过；公共底生产 pin 和补丁 donor map 仍为空，实际双架构资格未完成。接入还须确保 Node 安全补丁在平台发布阶段完成：未提供 donor 时，新源码的 `commonNodeBuildStage` 会在租户 Dockerfile 中编译补丁，这条尚未启用的后备路径不能作为 D1=A 的正式实现。应复用已审核的同版本补丁 donor，并以实际主运行时和 companion 生成镜像证明无需逐租户安全补丁编译。详见[复查与测试证据](evidence/common-integration-gate-after-37ec/MANIFEST.json)。

| 运行时 | 已验证的精确版本（候选 donor） |
| --- | --- |
| Node | 14.21.3, 16.20.2, 18.20.8, 20.20.2 |
| Python | 3.7.17, 3.8.20, 3.9.25, 3.10.22 |
| Ruby | 3.0.7, 3.1.7, 3.2.11 |

[完整交接 JSON](evidence/common-integration-handoff/common-integration-handoff.json)包含每个固定引用、producer/verifier、证明哈希、实际扫描处置、公共底资格条件和后续运行步骤；[原中文说明](evidence/common-integration-handoff/handoff.zh.txt)保留详细边界。

本项不会临时写入逐语言租户基础镜像。旧 collector-v2 和直接11-pin写入计划已退役；后续应按实际合格公共底与运行时模型重新审阅采集工具。旧清单中的五行未知对象及所有新 App 必须在真实新 operator Ready 后重新核对，旧报告、无 Pod 或发布数量都不能充当最终安全或线上验收。


2026-11-01协作代码已准备并通过完整operator与相关源控制，见[本轮实际记录](evidence/d2-common-collaboration-20261101/MANIFEST.json)。新的精确14.21.2发布入口修改了公共镜像的严格输入集合，旧56c58审阅只能保留历史范围；下一轮应使用新鲜发布/审阅和当前源码的完整双架构utilities，再进入donor/family及线上检查。手动新CI触发还需恢复可用本机GitHub登录或浏览器入口；已给用户具体登录动作，不把已同意协作、源码测试或旧run当作新资格通过。


本轮停止前的[原始观察、离线适配准备和根核对](evidence/d2-ci-dispatch-gate-20261101/MANIFEST.json)不增加产物采用权限；1feba草稿在main前移后必须按实际run新建source计划并审阅。旧26b fresh499706的utility成功保留为历史，不能替代当前源码资格。
