import type { TranslationEntry } from "@/i18n";

const koSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "SSH 공개 키",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "내 계정에 등록된 공개 키입니다. 비공개 키는 기기 밖으로 나가지 않습니다.",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "SSH 키 추가",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "SSH 공개 키 추가",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "OpenSSH 공개 키 하나를 붙여넣으세요. 저장할 때 주석은 제거됩니다.",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "이름",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "공개 키",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "지원되는 OpenSSH 공개 키 하나를 입력하세요(RSA는 2048비트 이상이어야 합니다).",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": {
    message: "취소",
    description: "Cancel SSH key action",
  },
  "sshKeys.save": {
    message: "키 추가",
    description: "Submit SSH key button",
  },
  "sshKeys.fingerprint": {
    message: "지문",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "생성일",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "작업",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "SSH 키 없음",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message: "실행 중인 서비스에 연결하기 전에 공개 키를 추가하세요.",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "SSH 키를 불러오지 못했습니다",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message:
      "다시 시도하세요. 접근이 거부되면 워크스페이스 관리자에게 문의하세요.",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "{name} 추가됨",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "이 SSH 키를 추가하지 못했습니다. 형식을 확인하고 2048비트 이상의 RSA 또는 그보다 강력한 키를 사용하세요.",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "이미 등록된 공개 키입니다",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": {
    message: "삭제",
    description: "Delete SSH key action",
  },
  "sshKeys.deleteNamed": {
    message: "SSH 키 {name} 삭제",
    description: "Row delete button aria-label identifying the SSH key",
  },
  "sshKeys.deleteTitle": {
    message: "'{name}' 키를 삭제할까요?",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message: "이 키를 사용하는 새 SSH 연결은 즉시 거부됩니다.",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "{name} 삭제됨",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "이 SSH 키를 삭제하지 못했습니다",
    description: "Delete SSH key failure toast",
  },
  "sshKeys.gateTitle": {
    message: "연결하려면 SSH 키를 추가하세요",
    description: "RequiresSshKey gate — CTA title when no key is registered",
  },
  "sshKeys.gateBody": {
    message:
      "이 연결은 본인의 키를 사용하는 SSH로 이루어집니다. 공개 키를 한 번 추가한 뒤 다시 돌아와 연결하세요.",
    description: "RequiresSshKey gate — CTA body",
  },
  "sshKeys.gateAddKey": {
    message: "SSH 키 추가",
    description: "RequiresSshKey gate — primary action linking to settings",
  },
};

export default koSSHKeys;
