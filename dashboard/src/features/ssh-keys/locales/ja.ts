import type { TranslationEntry } from "@/i18n";

const jaSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "SSH 公開鍵",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "あなたのアイデンティティに登録された公開鍵です。秘密鍵がお使いのデバイスの外に出ることはありません。",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "SSH キーを追加",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "SSH 公開鍵を追加",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "OpenSSH 形式の公開鍵を 1 つ貼り付けてください。コメントは保存時に削除されます。",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "名前",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "公開鍵",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "サポートされている OpenSSH 公開鍵を 1 つ入力してください（RSA は 2048 ビット以上が必要です）。",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": {
    message: "キャンセル",
    description: "Cancel SSH key action",
  },
  "sshKeys.save": {
    message: "キーを追加",
    description: "Submit SSH key button",
  },
  "sshKeys.fingerprint": {
    message: "フィンガープリント",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "作成日",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "操作",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "SSH キーがありません",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message: "実行中のサービスに接続する前に、公開鍵を追加してください。",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "SSH キーを読み込めませんでした",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message:
      "もう一度お試しください。アクセスが拒否される場合は、ワークスペースの管理者に確認してください。",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "{name} を追加しました",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "この SSH キーを追加できませんでした。形式を確認し、2048 ビット以上の RSA または同等以上の強度の鍵を使用してください。",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "この公開鍵はすでに登録されています",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": {
    message: "削除",
    description: "Delete SSH key action",
  },
  "sshKeys.deleteNamed": {
    message: "SSH キー {name} を削除",
    description: "Row delete button aria-label identifying the SSH key",
  },
  "sshKeys.deleteTitle": {
    message: "{name} を削除しますか？",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message: "このキーを使用する新しい SSH 接続は、ただちに拒否されます。",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "{name} を削除しました",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "この SSH キーを削除できませんでした",
    description: "Delete SSH key failure toast",
  },
  "sshKeys.gateTitle": {
    message: "接続するには SSH キーを追加してください",
    description: "RequiresSshKey gate — CTA title when no key is registered",
  },
  "sshKeys.gateBody": {
    message:
      "この接続は、お客様自身のキーを使用した SSH で行われます。公開鍵を一度追加してから、戻って接続してください。",
    description: "RequiresSshKey gate — CTA body",
  },
  "sshKeys.gateAddKey": {
    message: "SSH キーを追加",
    description: "RequiresSshKey gate — primary action linking to settings",
  },
};

export default jaSSHKeys;
