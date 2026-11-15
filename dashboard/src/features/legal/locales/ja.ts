import type { TranslationEntry } from "@/i18n";

const jaLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "利用規約",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "エージェントモデルの利用規約",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "{date} 発効",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "適用範囲",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "この規約は、ワークスペースのエージェントセッションが、ワークスペース独自のモデルプロバイダーのキーではなく、bex が提供するモデルを使用する場合に適用されます。",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "お客様のデータ",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "応答を生成するため、bex はセッションのプロンプト、コード、ツールの出力を、OpenRouter を通じてサードパーティのモデルプロバイダーに送信します。",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "リクエストは、処理を超えてプロンプトを保存せず、プロンプトで学習も行わないプロバイダーにのみルーティングされます。bex は、データ収集を拒否する設定の OpenRouter プロバイダールーティングを使用します。",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Muse Code のセッションは例外です。bex は Muse Code のセッションを、コントリビューターティアで Meta のモデル API に直接送信します。このティアでは、Meta は送信されたプロンプト、コード、ファイル、コンテキスト、および返された応答を、自社モデルの学習に使用する場合があります。Muse Code は、管理者がオンにしてこれに同意するまで、ワークスペースでは無効のままです。",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "ワークスペースが代わりに独自のプロバイダーキーを使用する場合、リクエストはお客様とプロバイダーとの契約に基づいてそのプロバイダーに送信され、bex はモデルのトークンを請求しません。",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "料金",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "各リクエストは、モデルプロバイダーの実費に 30% を加えた金額で請求されます。使用量はリクエストごとに計測され、請求 → エージェントモデルに表示されます。",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "支出上限",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "各ワークスペースには、プランに応じた月間の支出上限があります: Hobby は $20、Pro は $200、Scale は $2,000 です。ワークスペースの管理者は、プランの上限の最大 10 倍まで調整できます。",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "新しいワークスペースは、最初の支払いが成功するまで、bex が提供するモデルで同時に 2 つのセッションを実行できます。",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "上限に達すると、翌月が始まるか管理者が上限を引き上げるまで、新しいモデルリクエストは拒否されます。",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "月の途中での請求",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "お客様は、月次の請求書に加えて、未請求のエージェントモデルの使用量がワークスペースの請求しきい値に達するたびに、bex がワークスペースの支払い方法に請求することを承認するものとします。請求の処理中もワークスペースは引き続き動作します。しきい値は $10 から始まり、ワークスペースの支払い履歴に応じて上がります。",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "請求に失敗した場合、または認証が必要な場合、残高が支払われるまでエージェントモデルの使用は一時停止されます。",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "ワークスペースまたはアカウントの削除",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "ワークスペースまたはアカウントを削除する前に、bex は未請求の使用量を請求します。その請求に失敗した場合、残高が支払われるまで削除は保留されます。",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "お客様は、削除前に発生した使用量について、削除後も含めて bex が請求することを承認するものとします。",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "支払い方法",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "プリペイドカードは使用できません。未払いの bex の残高に紐づいたカードは、拒否される場合があります。",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "使用記録",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex は、すべてのリクエストの記録（時刻、モデル、トークン数、料金）を 13 か月間保管します。請求に異議が申し立てられた場合、これらの記録が証拠として使用されることがあります。",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "規約の変更",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "bex は、この規約を更新する場合があります。このページ上部の発効日が、現在のバージョンを示します。",
    description: "Terms: changes",
  },
};

export default jaLegal;
