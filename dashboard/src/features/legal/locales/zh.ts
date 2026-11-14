import type { TranslationEntry } from "@/i18n";

const zhLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "条款",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "智能体模型使用条款",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "生效日期：{date}",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "适用范围",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "当工作区的智能体会话使用 bex 提供的模型，而不是工作区自己的模型提供方密钥时，适用本条款。",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "你的数据",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "为生成回复，bex 会通过 OpenRouter 将会话的提示词、代码和工具输出发送给第三方模型提供方。",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "请求只会路由到在处理之外不存储提示词、也不使用其进行训练的提供方：bex 使用拒绝数据收集的 OpenRouter 提供方路由。",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Muse Code 会话是例外。bex 会将其直接发送到 Meta 模型 API 的贡献者层级，在该层级下，Meta 可能使用所发送的提示词、代码、文件和上下文以及返回的回复来训练其模型。在管理员开启 Muse Code 并确认这一点之前，它在工作区中保持关闭。",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "如果工作区改用自己的提供方密钥，请求将依据你与该提供方之间的协议发送给你的提供方，bex 不收取模型 token 费用。",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "价格",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "每个请求按模型提供方的实际成本加 30% 计费。用量按请求计量，并显示在 账单 → 智能体模型 中。",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "支出上限",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "每个工作区都有按套餐设定的每月支出上限：Hobby $20、Pro $200、Scale $2,000。工作区管理员最多可将其调整为套餐上限的 10 倍。",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "新工作区在首次成功付款之前，最多同时运行两个使用 bex 提供模型的会话。",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "达到上限后，新的模型请求将被拒绝，直到下个月开始或管理员提高上限。",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "月中扣款",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "除月度账单外，每当工作区未结算的智能体模型用量达到该工作区的扣款阈值时，你授权 bex 向工作区的付款方式扣款。扣款期间工作区可继续使用。阈值从 $10 起，并随工作区的付款记录逐步提高。",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "如果扣款失败或需要你完成验证，智能体模型的使用将暂停，直到欠款付清。",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "删除工作区或账户",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "在删除工作区或账户之前，bex 会对其未结算的用量扣款。如果该扣款失败，删除将等到欠款付清后再进行。",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message: "你授权 bex 对删除前产生的用量扣款，包括在删除之后扣款。",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "付款方式",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message: "不接受预付卡。与 bex 未付欠款关联的卡可能会被拒绝。",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "用量记录",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex 会将每个请求的记录（时间、模型、token 数和价格）保留 13 个月。如果某笔扣款产生争议，这些记录可作为证据使用。",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "条款变更",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message: "bex 可能会更新本条款。页面顶部的生效日期表示当前版本。",
    description: "Terms: changes",
  },
};

export default zhLegal;
