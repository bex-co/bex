import type { TranslationEntry } from "@/i18n";

const jaNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "通知",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "サービスのデプロイが開始・成功・失敗したときや、実行時に問題が発生したときにメールで通知を受け取ります。これは個人の設定で、ワークスペースの各メンバーがそれぞれ独立して設定します。",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "デプロイ開始",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message:
      "デプロイが開始されたらメールで通知します。すべての通知を送信する設定のサービスにのみ送信されます。",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "デプロイ成功",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message:
      "デプロイが公開されたらメールで通知します。すべての通知を送信する設定のサービスにのみ送信されます。",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "失敗",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message:
      "デプロイの失敗、サービスの異常、cron 実行の失敗、イメージ取得の失敗、Blueprint 同期の失敗をメールでお知らせします。",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "通知設定を読み込めませんでした",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "設定を保存できませんでした",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "モバイルプッシュ",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "bex 拡張機能",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "bex モバイルのアラート、緊急度、配信時間帯、サービス単位の例外を設定します。上記のメール設定は変更されません。",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "モバイルプッシュを有効にする",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "登録済みの bex モバイルデバイスにアラートを送信します。",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "この bex サーバーではプッシュ配信が設定されていません",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "ポリシーの準備と保存は今でもできますが、オペレーターがプッシュプロバイダーを設定するまで、登録済みのデバイスは通知を受信できません。",
    description: "Disabled server status hint",
  },
  "notifications.webPushTitle": {
    message: "ブラウザー通知",
    description: "Web push settings card title",
  },
  "notifications.webPushDescription": {
    message:
      "このブラウザーでも同じアラートを受け取れます。イベントフィルターとサイレント時間は「モバイルプッシュ」で設定したものが使われ、別のポリシーはありません。",
    description: "Web push settings description",
  },
  "notifications.webPushUnsupported": {
    message: "このブラウザーは Web プッシュに対応していません。",
    description: "Unsupported browser",
  },
  "notifications.webPushUnconfigured": {
    message: "この bex サーバーではブラウザープッシュが有効になっていません。",
    description: "Server VAPID unset",
  },
  "notifications.webPushDenied": {
    message:
      "このサイトの通知はブロックされています。ブラウザーの設定で許可してから、もう一度お試しください。",
    description: "Permission denied",
  },
  "notifications.webPushError": {
    message: "ブラウザー通知を更新できませんでした。もう一度お試しください。",
    description: "Subscribe error",
  },
  "notifications.webPushOn": {
    message: "このブラウザーで bex のアラートを受信します。",
    description: "Subscribed state",
  },
  "notifications.webPushOff": {
    message: "このブラウザーは登録されていません。",
    description: "Unsubscribed state",
  },
  "notifications.webPushEnable": {
    message: "このブラウザーで有効にする",
    description: "Subscribe button",
  },
  "notifications.webPushDisable": {
    message: "このブラウザーで無効にする",
    description: "Unsubscribe button",
  },
  "notifications.pushEvents": {
    message: "イベントフィルター",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message:
      "選択したイベントタイプのうち、最小緊急度以上のものだけがプッシュ通知の対象になります。",
    description: "Push events hint",
  },
  "notifications.pushEventNotSentYet": {
    message: "未送信: 現在このイベントを送信する機能はありません。",
    description:
      "Under a push event that no part of bex emits yet (it is reserved for an upcoming feature); selecting it is kept but has no effect",
  },
  "notifications.pushEventNotSentYetShort": {
    message: "未送信",
    description:
      "Compact form of the not-sent-yet note, under a push event in a per-service override's event grid",
  },
  "notifications.pushEventFiltered": {
    message: "最小緊急度が {minimum} の間はプッシュされません。",
    description:
      "Under a selected push event whose urgency is below the minimum ({minimum} = urgency name, e.g. 'Important')",
  },
  "notifications.pushFilteredTitle": {
    message: "選択した一部のイベントはプッシュされません",
    description:
      "Heading of the save-area summary listing selected push events the minimum urgency drops",
  },
  "notifications.pushFilteredEvent": {
    message:
      "{event} は {urgency} のため、最小緊急度が {minimum} の間はプッシュされません。",
    description:
      "One summary line ({event} = event name, {urgency} = its urgency, {minimum} = the minimum urgency)",
  },
  "notifications.pushFilteredServiceEvent": {
    message:
      "{service}: {event} は {urgency} のため、このサービスの最小緊急度が {minimum} の間はプッシュされません。",
    description:
      "One summary line for a service override ({service} = service name; others as pushFilteredEvent)",
  },
  "notifications.pushOverrideFiltered_other": {
    message:
      "このサービスの最小緊急度が {minimum} の間は、選択した {count} 件のイベントがプッシュされません。",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushEventDeployFailed": {
    message: "デプロイ失敗",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "サービスがクラッシュ",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "cron 実行の失敗",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "デプロイ開始",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "デプロイ成功",
    description: "Push event",
  },
  "notifications.pushEventServerAvailable": {
    message: "サービスが復旧",
    description: "Push event",
  },
  "notifications.pushEventServiceSuspended": {
    message: "サービスを一時停止",
    description: "Push event",
  },
  "notifications.pushEventServiceResumed": {
    message: "サービスを再開",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "エージェントの判断が必要",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "エージェントの PR の準備完了",
    description: "Push event",
  },
  "notifications.pushEventAgentFailed": {
    message: "エージェントセッションの失敗",
    description: "Push event",
  },
  "notifications.pushEventAgentAutomationPaused": {
    message: "オートメーションを一時停止",
    description: "Push event: the platform paused an agent automation",
  },
  "notifications.pushEventPostgresUnavailable": {
    message: "データベースが利用不可",
    description: "Push event",
  },
  "notifications.pushEventPostgresAvailable": {
    message: "データベースが復旧",
    description: "Push event",
  },
  "notifications.pushEventKeyValueUnhealthy": {
    message: "Key Value が異常",
    description: "Push event",
  },
  "notifications.pushEventKeyValueAvailable": {
    message: "Key Value が復旧",
    description: "Push event",
  },
  "notifications.pushEventPostgresBackupFailed": {
    message: "データベースのバックアップ失敗",
    description: "Push event",
  },
  "notifications.pushEventPostgresRestoreFailed": {
    message: "データベースのリストア失敗",
    description: "Push event",
  },
  "notifications.pushEventPostgresUpgradeFailed": {
    message: "データベースのアップグレード失敗",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "最小緊急度",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "通常",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "重要",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "緊急",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "IANA タイムゾーン",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "最大保留時間（時間）",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "稼働時間",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message:
      "設定すると、緊急以外のアラートはいずれかの時間帯になるまで保留されます。",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "サイレント時間",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message: "サイレント時間帯の間、緊急以外のアラートは保留されます。",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "月",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "火",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "水",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "木",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "金",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "土",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "日",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "時間帯の開始",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "時間帯の終了",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "時間帯を削除",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "時間帯を追加",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "サービス別の上書き",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "省略した項目は上記の設定を引き継ぎます。イベントリストを明示的に空にすると、そのサービスはミュートされます。",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "サービスの上書きを削除",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "配信",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "継承",
    description: "Override option",
  },
  "notifications.pushOn": {
    message: "オン",
    description: "Override option",
  },
  "notifications.pushOff": {
    message: "オフ",
    description: "Override option",
  },
  "notifications.pushExactEvents": {
    message: "イベントリストを明示して上書き",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "サービスの上書きを追加…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "プッシュ設定を保存",
    description: "Save button",
  },
  "notifications.pushUnsaved": {
    message: "未保存の変更があります",
    description:
      "Hint beside Save push settings while the push form differs from the saved settings (w4/435)",
  },
  "notifications.pushDiscardTitle": {
    message: "プッシュ設定の変更を破棄しますか？",
    description:
      "Navigation guard title when the push form has unsaved changes (w4/435)",
  },
  "notifications.pushDiscardBody": {
    message: "変更は保存されていないため、移動すると失われます。",
    description: "Navigation guard explanation for unsaved push settings",
  },
  "notifications.pushKeepEditing": {
    message: "編集を続ける",
    description: "Cancel the navigation and stay on the push form",
  },
  "notifications.pushDiscard": {
    message: "破棄して移動",
    description: "Confirm leaving and dropping unsaved push settings",
  },
  "notifications.pushSaved": {
    message: "プッシュ設定を保存しました",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "プッシュ設定を保存できませんでした",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "プッシュ設定を読み込めませんでした",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message:
      "America/Los_Angeles のような有効な IANA タイムゾーンを入力してください。",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "最大保留時間は 0.5 〜 168 時間の範囲で指定してください。",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "イベントフィルターにサポートされていない値が含まれています。",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "有効な緊急度を選択してください。",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "すべての時間帯に、曜日、有効な開始・終了時刻、および異なる開始と終了が必要です。",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message: "スケジュールの時間帯またはサービスの上書きが多すぎます。",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message:
      "サービスの上書きは重複せず、有効なサービスを指定する必要があります。",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message:
      "各サービスの上書きでは、配信、緊急度、イベントのいずれかを変更する必要があります。",
    description: "Empty override validation",
  },
  "notifications.workspaceTitle": {
    message: "通知",
    description:
      "Workspace Settings → Notifications card title (the workspace notification policy)",
  },
  "notifications.workspaceDescription": {
    message:
      "すべてのサービスの既定値です。サービスごとに、その設定で上書きできます。",
    description: "Workspace Settings → Notifications card description",
  },
  "notifications.workspaceReadOnly": {
    message: "これらの既定値を変更できるのはワークスペースの管理者のみです。",
    description: "Workspace notification policy: read-only note for non-admins",
  },
  "notifications.workspaceEmailLabel": {
    message: "メール通知",
    description: "Workspace notification policy: email destination switch",
  },
  "notifications.workspaceEmailHint": {
    message:
      "オフにすると、サービスの設定に関係なく通知メールは送信されません。",
    description: "Workspace notification policy: email destination hint",
  },
  "notifications.workspaceLevelLabel": {
    message: "サービス通知の既定値",
    description: "Workspace notification policy: level radio group label",
  },
  "notifications.workspaceLevelHint": {
    message: "ワークスペースの既定値を使うサービスが送信する内容です。",
    description: "Workspace notification policy: level radio group hint",
  },
  "notifications.levelAll": {
    message: "すべての通知",
    description: "Notification level: every notification",
  },
  "notifications.levelAllHint": {
    message: "失敗に加えて、デプロイの開始と公開、復旧も通知します。",
    description: "Notification level hint: all",
  },
  "notifications.levelFailure": {
    message: "失敗の通知のみ",
    description: "Notification level: failures only (Render's default)",
  },
  "notifications.levelFailureHint": {
    message:
      "デプロイの失敗、サービスの異常、cron 実行の失敗、ディスクの満杯。",
    description: "Notification level hint: failure",
  },
  "notifications.levelNone": {
    message: "なし",
    description: "Notification level: nothing",
  },
  "notifications.levelNoneHint": {
    message: "これらのサービスの通知を送信しません。",
    description: "Notification level hint: none",
  },
  "notifications.workspaceSave": {
    message: "保存",
    description: "Workspace notification policy: save button",
  },
  "notifications.workspaceSaved": {
    message: "通知の既定値を保存しました",
    description: "Workspace notification policy: success toast",
  },
  "notifications.workspaceSaveFailed": {
    message: "通知の既定値を保存できませんでした。もう一度お試しください。",
    description: "Workspace notification policy: save error",
  },
  "notifications.workspaceLoadFailed": {
    message: "ワークスペースの通知の既定値を読み込めませんでした。",
    description: "Workspace notification policy: load error",
  },
  "notifications.workspaceRetry": {
    message: "再試行",
    description: "Workspace notification policy: retry button",
  },
  "notifications.overridesTitle": {
    message: "通知の上書き",
    description:
      "Workspace notifications: overrides list title (Render's label)",
  },
  "notifications.overridesDescription": {
    message: "通知設定がワークスペースの既定値と異なるサービスです。",
    description: "Workspace notifications: overrides list description",
  },
  "notifications.overridesEmpty": {
    message: "ワークスペースの既定値を上書きしているサービスはありません。",
    description: "Workspace notifications: no overrides",
  },
  "notifications.overridesError": {
    message: "既定値を上書きしているサービスを読み込めませんでした。",
    description: "Workspace notifications: overrides failed to load",
  },
};

export default jaNotifications;
