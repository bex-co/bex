import type { TranslationEntry } from "@/i18n";

const ja: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "サンドボックス",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "スナップショット",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "カスタムイメージ",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "設定",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "作成中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "実行中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "再開中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "一時停止処理中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "一時停止中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "エラー",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "終了済み",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "期間",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "過去 1 時間",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "過去 24 時間",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "過去 7 日間",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "過去 30 日間",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "全期間",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "名前、サンドボックス ID、ステータスで検索",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "スナップショットを検索",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "イメージを検索",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "ライブ更新を一時停止",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "ライブ更新を再開",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "列の表示/非表示",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "合計",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "エラー",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message: "期間内にエラーで終了したサンドボックスの割合です。",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "実行中",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "期間にかかわらず、現在作成中、実行中、再開中、または一時停止処理中のサンドボックスです。これらは同時実行数の上限に含まれます。",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "サンドボックスの実行記録を読み込めませんでした",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message:
      "終了したサンドボックスの実行: 成功 {succeeded} 件、エラー {errored} 件",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "成功",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "エラー",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "{count} 個のサンドボックスを選択中",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "終了",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "{count} 個のサンドボックスを終了しますか？",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "{count} 個のサンドボックスが停止し、スナップショットに含まれていない内容はすべて失われます。",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "終了",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message:
      "{count} 個のサンドボックスを終了できませんでした。もう一度お試しください。",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "{count} 個のサンドボックスを終了しています",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "サンドボックスを終了",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "サンドボックスの操作",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "サンドボックスを読み込めませんでした",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "サンドボックスを読み込めませんでした",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "一致するサンドボックスはありません",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "別のキーワードで検索するか、期間を広げてください。",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "フィルターをクリア",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "最初のサンドボックスを作成",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "CLI からサンドボックスを起動するか、TypeScript または Python の SDK でプログラムからサンドボックスを作成するようにコードを設定してください。",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "最新の CLI をインストールまたは更新",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "直接ダウンロード",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "ログインしてこのワークスペースを使用",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "デフォルト設定でサンドボックスを作成",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "API キーを設定",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "コードを実行するターミナルでこのコマンドを実行してください。SDK は RENDER_API_KEY 環境変数からキーを読み取ります。プレースホルダーを bex の API キーに置き換えてください。",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "プロジェクトを準備して SDK をインストール",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Python 3.10 以降が必要です。-U を付けると古い SDK がアップグレードされます。1.4.0 以前はサンドボックスに対応していません。",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Node.js 18 以降が必要です。@latest を付けると、インストール済みの古い SDK もアップグレードされます。",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "最初のサンドボックスを初期化",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "コードをローカルで実行",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": {
    message: "サンドボックス",
    description: "Column",
  },
  "sandboxGroupPages.colStatus": {
    message: "ステータス",
    description: "Column",
  },
  "sandboxGroupPages.colCreated": {
    message: "作成日時",
    description: "Column",
  },
  "sandboxGroupPages.colDuration": {
    message: "実行時間",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "プラン", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "起動元",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "このページのすべてのサンドボックスを選択",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "{name} を選択",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "ベースイメージ",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "インポートしたイメージ",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "前へ",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "次へ",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message:
      "選択していたサンドボックスが停止したため、実行中のサンドボックスに切り替えました。",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": {
    message: "コピーしました",
    description: "Copy toast",
  },
  "sandboxGroupPages.copyError": {
    message: "コピーできませんでした",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "サンドボックス ID をコピー",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "スナップショット ID をコピー",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "イベント",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabSnapshots": {
    message: "スナップショット",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "詳細",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "バージョン",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "サンドボックスが見つかりません",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id} は、このグループで表示できるサンドボックスではありません。",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "イベントを読み込み中",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message:
      "イベントを読み込めませんでした。しばらくしてからもう一度お試しください。",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "イベントはまだありません。",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "以前のイベントを読み込む",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "以前のイベントを読み込み中",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "作成",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "準備完了",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "一時停止処理中",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "一時停止",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "再開中",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "終了",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "エラー",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "コマンド（{operation}）",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "ファイル転送（{operation}）",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "終了コード {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "スナップショット",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "スナップショットを表示",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "ダッシュボード",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "アイドルタイムアウト",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "新しいコマンドまたはファイル転送",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "読み込み中",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "スナップショットを読み込めませんでした",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "このサンドボックスのスナップショットはまだありません。CLI または SDK で取得してください。",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "このサンドボックスのスナップショットは取得されませんでした。",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "サンドボックス ID",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "グループ",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "プラン",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan}（CPU {cpus} コア、メモリ {memory} GB）",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "作成日時",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "終了日時",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "実行時間",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "タイムアウト",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "アイドルタイムアウト",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "オフ",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "ネットワーク",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "すべて許可",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "すべて拒否",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "許可リスト",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "リージョン",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "起動元",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "スナップショットへ移動",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "イメージへ移動",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "トリガー",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "ユーザー",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "エラー",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex はこのサンドボックスでエラーが発生した原因を特定できませんでした。もう一度お試しいただくか、サンドボックス ID を添えてサポートにお問い合わせください。",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "取得中",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "利用可能",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "失敗",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "内部エラーにより取得が停止しました。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "この取得はタイムアウトしました。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message: "取得が完了する前に、元のサンドボックスが終了しました。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message: "取得が完了する前に、元のサンドボックスでエラーが発生しました。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "再試行するには新しいスナップショットを作成してください。このスナップショットから新しいサンドボックスは起動できません。",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "一致するスナップショットはありません",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "最初のスナップショットを作成",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "スナップショットを取得して、サンドボックスのファイルを保存します。すべてを設定し直すことなく、同じ時点から新しいサンドボックスを起動できます。",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message:
      "スナップショットを作成するには、実行中のサンドボックスが必要です。",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "実行中のサンドボックスのスナップショットを取得",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "スナップショットからサンドボックスを作成",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "コードを設定",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": {
    message: "スナップショット",
    description: "Column",
  },
  "sandboxGroupPages.colSourceSandbox": {
    message: "元のサンドボックス",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "サイズ",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "取得日時",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "有効期限",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "なし",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "スナップショットが見つかりません",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "{id} は、このワークスペースで表示できるスナップショットではありません。削除されたか、有効期限が切れた可能性があります。",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "タイプ",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "ファイルシステム",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "ランタイム",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "リソース制約",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "ライフサイクル",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "削除するまで保持",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "最後の使用から {days} 日後に期限切れ",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "このスナップショットから起動したサンドボックスはありません。",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "このスナップショットからサンドボックスを起動",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "スナップショットを削除",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "スナップショットを削除しますか？",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "{name} を完全に削除します。新しいサンドボックスはこのスナップショットから起動できなくなります。すでにこのスナップショットから起動したサンドボックスは引き続き実行されます。",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "スナップショットを削除できませんでした。もう一度お試しください。",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "スナップショット {name} を削除しました",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "待機中",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "インポート中",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "利用可能",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "利用不可",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "失敗",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "最新",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "このバージョンが完了するまで、サンドボックスは利用可能な最新バージョンから起動します。",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "インポートに時間がかかりすぎました。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "何度か再試行しましたが、インポートに失敗しました。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message:
      "bex はイメージをプルできませんでした。レジストリ認証情報を確認してください。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message:
      "bex はイメージを見つけられませんでした。イメージ名とタグを確認してください。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex はイメージをプルできませんでした。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message:
      "このイメージは {platform} 用です。サンドボックスは linux/amd64 にのみ対応しています。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "このイメージには、サンドボックスの起動に必要な {what} がありません。slim や Alpine のバリアントなど、シェルを含むイメージを使用してください。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message:
      "bex はイメージを変換できませんでした。もう一度インポートしてください。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex はイメージのレイヤーを読み取れませんでした。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "インポートを開始できませんでした。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "インポートは完了する前に停止しました。",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "イメージを読み込めませんでした",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "一致するイメージはありません",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "最初のイメージをインポート",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "レジストリからイメージをインポートすると、そのイメージからサンドボックスを起動できます。CLI または SDK でイメージをインポートしてから、サンドボックスを起動してください。",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "イメージをインポート（linux/amd64）",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "イメージからサンドボックスを作成",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "イメージをインポート",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "イメージ", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "ソース",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "利用可能な最新版",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "更新日時",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "イメージが見つかりません",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "このグループに {name} という名前のイメージはありません。",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "利用可能な最新バージョン",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "ソース",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "イメージ",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "ダイジェスト",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "バージョン ID",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "このイメージから起動したサンドボックスはありません。",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "一般",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "デフォルトにする",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} がデフォルトのサンドボックスグループになりました",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "保存", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "同時実行数の上限を保存しました",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "アウトバウンド認証情報",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "サンドボックスグループを削除",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message:
      "このグループを、サンドボックス、スナップショット、イメージ、認証情報とともに削除します。",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "ワークスペースのデフォルトグループは削除できません。先に別のグループをデフォルトにしてください。",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "サンドボックスグループを削除しますか？",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "サンドボックスグループ {name} は、実行履歴、ログ、設定とともに完全に削除されます:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "SDK、CLI、API でこのグループを使用しているものはすべて動作しなくなります。",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "サンドボックスグループ {name} を削除しました",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message:
      "リンクを開けませんでした。しばらくしてからもう一度お試しください。",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "サンドボックスグループへ移動",
    description: "Button",
  },
};

export default ja;
