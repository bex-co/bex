import type { TranslationEntry } from "@/i18n";

const zhSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "SSH 公钥",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message: "注册到您身份的公钥。私钥始终保留在您的设备上。",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "添加 SSH 密钥",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "添加 SSH 公钥",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message: "粘贴一个 OpenSSH 公钥。保存时会移除注释。",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "名称",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "公钥",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message: "请输入一个受支持的 OpenSSH 公钥（RSA 至少为 2048 位）。",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": { message: "取消", description: "Cancel SSH key action" },
  "sshKeys.save": { message: "添加密钥", description: "Submit SSH key button" },
  "sshKeys.fingerprint": {
    message: "指纹",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "创建时间",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "操作",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "没有 SSH 密钥",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message: "连接运行中的服务前，请先添加公钥。",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "无法加载 SSH 密钥",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message: "请重试；如果访问被拒绝，请联系工作区管理员。",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "已添加 {name}",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message: "无法添加此 SSH 密钥。请检查格式，并使用至少 2048 位的 RSA 密钥。",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "此公钥已注册",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": { message: "删除", description: "Delete SSH key action" },
  "sshKeys.deleteTitle": {
    message: "删除 {name}？",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message: "使用此密钥的新 SSH 连接将立即被拒绝。",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "已删除 {name}",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "无法删除此 SSH 密钥",
    description: "Delete SSH key failure toast",
  },
};

export default zhSSHKeys;
