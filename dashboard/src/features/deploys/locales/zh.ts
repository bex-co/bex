import type { TranslationEntry } from "@/i18n";

const zhDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "已创建",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "排队中",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "构建中",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "构建失败",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "预部署进行中",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "预部署失败",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "进行中",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "失败",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "已上线",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "已取消",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "已停用",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "未知",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "部署状态",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.imageDigestCopy": {
    message: "复制镜像摘要",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "已复制镜像摘要",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "无法复制到剪贴板",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.created": {
    message: "创建于",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "更新时间",
    description: "部署最近一次已存储状态转换时间的标签",
  },
  "deploys.started": {
    message: "开始于",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "完成于",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "耗时",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "耗时 {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "计时中",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "进行中",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "{timestamp}部署",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "{timestamp}创建",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "{timestamp}取消",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "{timestamp}失败",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "首次部署",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "API 部署",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "手动部署",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "回滚至 {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "部署钩子",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "蓝图同步",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "新提交",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "配置变更",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "设置变更",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "重启",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "平台重建",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "搜索日志…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "日志类型",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "全部日志",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "应用日志",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "构建日志",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "历史构建日志不可用",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message: "此控制台尚未连接持久日志存储。该部署的应用日志仍可能显示在这里。",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message: "无法刷新此部署。正在显示上次加载的详情。",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message: "此时间范围只加载了一部分。加载更早的日志以搜索更早的行。",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "该部署在预部署步骤结束前已取消，且未记录该步骤的完成时间，因此其之后的输出可能不在此时间范围内。请在日志选项中选择更大的范围查看。",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "无法确认该部署的预部署步骤何时结束，因此之后的步骤输出可能不在此时间范围内。请重试，或在日志选项中选择更大的范围。",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "状态时间线",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "已创建部署",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "部署已开始",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "部署进行中",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "部署已上线",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "部署失败",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "部署已取消",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "部署已停用",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message: "服务事件不可用；仅显示部署时间戳。",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.listTitle": {
    message: "部署",
    description: "Deploys tab: card title over the deploy-history list",
  },
  "deploys.columnDeploy": {
    message: "部署",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "触发方式",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "耗时",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "操作",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "暂无部署。",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "没有符合所选状态的部署。",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "按状态筛选",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "全部状态",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "搜索已加载的部署和提交…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "搜索已加载的部署和提交",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listEmptySearch": {
    message: "已加载的部署中没有匹配项。",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_other": {
    message: "{count} 个部署",
    description: "Deploys tab: complete deploy count",
  },
  "deploys.listCountLoaded_other": {
    message: "已加载 {count} 个部署",
    description: "Deploys tab: loaded count when more server pages remain",
  },
  "deploys.listLoadMore": {
    message: "加载更多",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "无法加载更早的部署记录。",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logOptions": {
    message: "日志选项",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "时间范围",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "部署时间窗",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "最近 15 分钟",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "最近 1 小时",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "最近 6 小时",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "最近 24 小时",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "最近 7 天",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "自动换行",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "显示时间戳",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "最大化",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "退出全屏",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
  "deploys.reason.http_port_waiting": {
    message: "正在等待容器启动，然后检测其 HTTP 端口",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "正在检测容器的 HTTP 端口",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message: "检测到的 HTTP 端口超出了服务 75 个端口的上限；请显式设置 PORT",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "已在端口 {port} 上检测到 HTTP，将通过该端口提供服务",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message: "端口 {port} 已接受连接，但尚未响应 HTTP；容器启动期间将继续扫描",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message: "还没有端口响应 HTTP；容器启动期间将继续扫描",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "没有端口响应 HTTP；改用后备端口 {port}。如果 HTTP 监听器绑定在其他端口，请设置 PORT",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_not_found": {
    message:
      "在镜像仓库中找不到镜像 {image}：仓库或标签不存在。请检查镜像 URL 和标签，或部署一个存在的镜像摘要",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} 拒绝了此服务用于镜像 {image} 的镜像仓库凭据（HTTP {status}）。请检查凭据的用户名和令牌，或从此服务移除该凭据以匿名拉取公开镜像",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "镜像仓库拒绝访问镜像 {image}。如果镜像是私有的，请为此服务添加 {host} 的镜像仓库凭据",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "镜像仓库拒绝读取镜像 {image}（HTTP {status}）。请检查镜像 URL 和标签",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message: "构建未在构建时限内完成；请查看构建日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message: "预部署命令未在其时限内完成；请查看预部署日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message: "部署未能在健康检查时限内通过健康检查；请查看服务日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "容器正在运行，但其{kind}健康检查尚未成功，发布正在等待：端口 {port} 上的 GET {path}。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "容器正在运行，但其{kind}健康检查尚未成功，发布正在等待：端口 {port} 上的 GET {path}。存活检查已重启容器 {restarts} 次，说明检查确实在失败，而不只是响应慢。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "容器正在运行，但其{kind}健康检查在发布时限内始终未成功，部署因此失败：端口 {port} 上的 GET {path}。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "容器正在运行，但其{kind}健康检查在发布时限内始终未成功，部署因此失败：端口 {port} 上的 GET {path}。存活检查已重启容器 {restarts} 次，说明检查确实在失败，而不只是响应慢。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "容器正在运行，但其{kind}健康检查尚未成功，发布正在等待：到端口 {port} 的 TCP 连接。未设置健康检查路径，因此这项检查只是一次 TCP 连接，只能确认进程是否在该端口上监听。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "容器正在运行，但其{kind}健康检查尚未成功，发布正在等待：到端口 {port} 的 TCP 连接。存活检查已重启容器 {restarts} 次，说明检查确实在失败，而不只是响应慢。未设置健康检查路径，因此这项检查只是一次 TCP 连接，只能确认进程是否在该端口上监听。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "容器正在运行，但其{kind}健康检查在发布时限内始终未成功，部署因此失败：到端口 {port} 的 TCP 连接。未设置健康检查路径，因此这项检查只是一次 TCP 连接，只能确认进程是否在该端口上监听。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "容器正在运行，但其{kind}健康检查在发布时限内始终未成功，部署因此失败：到端口 {port} 的 TCP 连接。存活检查已重启容器 {restarts} 次，说明检查确实在失败，而不只是响应慢。未设置健康检查路径，因此这项检查只是一次 TCP 连接，只能确认进程是否在该端口上监听。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "发布未能在进度时限内进入健康状态",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message: "预部署命令未在 {minutes} 分钟的时限内完成；请查看预部署日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message: "预部署命令内存不足（退出码 {code}）；请查看预部署日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message: "预部署命令以退出码 {code} 退出；请查看预部署日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message: "预部署命令失败；请查看预部署日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "预部署命令无法启动：容器运行时无法运行其程序；该程序从未运行，因此没有预部署日志",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "构建一直未开始：{cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message: "构建未在构建时限内完成，且无法读取此服务的 App 记录来说明原因",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message: "部署未在其时限内完成，且无法读取此服务的 App 记录来说明原因",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "由于平台错误，bex 无法应用此次发布；这不是你的代码或设置的问题。请联系支持团队并提供参考编号 {ref}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "已取消：服务已暂停",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "已被较新的发布取代",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "已被 {deploy} 取代",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "已被重启 {deploy} 取消，该重启保留了正在运行的发布：此部署要应用的内容未被部署",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "已随 {deploy} 一同取消",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "已随一个较新的部署一同取消",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "该服务已不存在",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message: "这次重启等待的部署未在 {wait}内完成",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "容器正在运行，但其{kind}健康检查尚未成功，发布正在等待：已配置的健康检查。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "容器正在运行，但其{kind}健康检查尚未成功，发布正在等待：已配置的健康检查。存活检查已重启容器 {restarts} 次，说明检查确实在失败，而不只是响应慢。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "容器正在运行，但其{kind}健康检查在发布时限内始终未成功，部署因此失败：已配置的健康检查。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "容器正在运行，但其{kind}健康检查在发布时限内始终未成功，部署因此失败：已配置的健康检查。存活检查已重启容器 {restarts} 次，说明检查确实在失败，而不只是响应慢。请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      "端口 {port} 上没有任何进程接受 TCP 连接；镜像未声明任何 TCP 端口，因此使用的是默认端口。服务器必须绑定到 0.0.0.0，并监听 PORT 环境变量指定的端口（{port}）。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      "端口 {port} 上没有任何进程接受 TCP 连接，它是从镜像声明的 TCP 端口（{ports}）中选出的主端口。请让服务器在其中一个端口上绑定 0.0.0.0，或把服务的端口设为它实际监听的端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      "端口 {port} 上没有任何进程接受 TCP 连接，该端口由服务的 PORT 环境变量设定；镜像未声明其他 TCP 端口。请让服务器在端口 {port} 上绑定 0.0.0.0，或把服务的端口改为它实际监听的端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      "端口 {port} 上没有任何进程接受 TCP 连接，该端口是服务配置的端口；镜像未声明其他 TCP 端口。请让服务器在端口 {port} 上绑定 0.0.0.0，或把服务的端口改为它实际监听的端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      "端口 {port} 上没有任何进程接受 TCP 连接，该端口由服务的 PORT 环境变量设定；镜像声明了 TCP 端口（{ports}）。请把 PORT 环境变量改为它实际监听的端口，或删除该变量以使用镜像的端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      "端口 {port} 上没有任何进程接受 TCP 连接，该端口是服务配置的端口；镜像声明了 TCP 端口（{ports}）。请在设置中把服务的端口改为它实际监听的端口，或删除该设置以使用镜像的端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message: "如果服务不需要在任何端口上接收流量，请改为创建后台工作进程。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message: "启动探测期间没有端口响应 HTTP。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: "进程正在端口 {port} 上监听。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message: "进程正在端口 {port} 上监听，服务现已改用该端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      "进程正在端口 {ports} 上监听，而不是端口 {port}；请把服务的端口（或其 PORT 变量）设为提供 HTTP 服务的那个端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message: "进程正在端口 {port} 上监听，服务现已将其用作主端口。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      "实例上未检测到开放端口：服务器必须绑定到 0.0.0.0，并监听 PORT 环境变量指定的端口（{port}）。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      "实例上未检测到开放端口：服务器必须绑定到 0.0.0.0，并监听 PORT 环境变量指定的端口（{port}）；它之前监听的是端口 {previous}。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      "实例上未检测到开放端口：服务器必须绑定到 0.0.0.0，并监听服务的端口（{port}）。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message: "{noun}已有 {active}/{limit} 个并发构建在运行；正在等待空位",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message: "集群的构建容量已满；正在等待空位",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message: "同时发起的另一个构建占用了最后一个构建空位；正在等待空位",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message: "正在等待关联的数据库 {database} 可用，然后再开始构建",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message: "正在等待关联数据存储的连接信息，然后再开始构建",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message: "正在等待镜像仓库接受此应用的构建凭据",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message: "正在等待镜像仓库接受此应用的拉取凭据",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "正在等待构建资源：{detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message: "正在等待构建资源：构建任务已派发，正在等待创建其构建 Pod",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message: "正在等待构建资源：暂无空闲的构建节点",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message: "正在等待构建资源：暂无空闲的构建节点（可用的 {resources} 不足）",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "启动",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "就绪",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "工作区",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} 小时",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} 分钟",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} 秒",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
};

export default zhDeploys;
