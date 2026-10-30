# D2 上游提交：w8/103

- **PR：** [Preserve literal local paths in secret-file arguments](https://github.com/render-oss/cli/pull/61)。
- **基线 / commit：** `c31e586eb6f38138af3cc986b079e46a78c61de7` / `983f1db802e857d8ba930d7923d77a4ebcc52f79`。
- **授权：** 用户明确允许提交对 Render CLI 有帮助的修复；本提交只改 Go 产品代码/测试，没有 workflow、生成 client 或第三方依赖变更。
- **验证：** 57 个开发包通过；适用的完整组合分支也重跑通过。相对原版 main 的新增 lint 为 0；原版在固定 golangci-lint v2.14.0 下有 92 项既有告警。真实 Render API 的 e2e 按上游 AGENTS.md 不在默认开发验证范围内。
- **边界：** PR 提交不等于 upstream 合入/发布或 Bex 交付。原卡片的 adoption、发布、native/shared-surface 和清理验收均保留。没有把这些卡片标 done。

[可复现补丁](upstream.patch) · [版本、文件与验证回执](validation.json)。
