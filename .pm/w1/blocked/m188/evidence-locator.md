# 证据定位与历史路径

本轮按真实公共底上游依赖将里程碑停放到 `blocked/m188`，不是完成或删除。t001/t002保持完成，t003–t008的实际集成与验收继续开放。

- 原 `.pm/w1/m188/` 前缀映射到 `.pm/w1/blocked/m188/`；已完成任务保留 `done/` 层级。不要在旧位置新建stub或symlink。
- 用“原始路径＋期望SHA-256＋字节数”识别证据，在各MANIFEST或PORTABLE-LOCATORS中查找对应内容。同一旧 `/tmp` 路径的最新文件可能属于另一历史版本。
- `archive_path` 相对于所属MANIFEST目录；整个里程碑在blocked/done间迁移时，内部与同级清单相对路径保持有效。证据中的Markdown、JSON、源码、原始argv与绝对路径一律保留原文。
- gzip的stored/gzip哈希校验磁盘文件，original/raw哈希校验解压内容；不能用压缩哈希代替原始哈希，也不能格式化、改行尾或重构签名语句。
- 历史工具可能固定旧绝对路径。恢复验证前需审阅当前路径解析或隔离验证环境，按清单定位正确字节并保留源码、签名与subject检查；不要为了运行旧工具复建本仓库已移走的active目录。公共底实际接入需要重新审阅采集工具，旧逐语言写入器已退役。
- 大型完整签名谓词仍由不可变公共OCI对象及已核对的本地gzip缓存保管，索引见[Node18完整报告定位](evidence/node18-published-acceptance/LARGE-PREDICATES-EXTERNAL.json)。未重复存入Git，也不据此宣称新的下载、公共底资格或线上验收。
