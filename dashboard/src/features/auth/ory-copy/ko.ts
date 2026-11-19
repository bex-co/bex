import type { OryCopy } from "./keys";

// Ory Elements' own `ko` catalog entries, copied verbatim (see ./keys.ts).
const ko: OryCopy = {
  "orySettings.profileTitle": "프로필 설정",
  "orySettings.profileDescription": "프로필 정보 업데이트",
  "orySettings.passwordTitle": "비밀번호 변경",
  "orySettings.passwordDescription": "비밀번호 수정",
  "orySettings.passwordLabel": "비밀번호",
  "orySettings.oidcTitle": "연결된 계정",
  "orySettings.oidcDescription": "소셜 로그인 제공자와 계정을 연결하세요.",
  "orySettings.oidcInfo":
    "이 제공자에서 연결된 계정은 계정에 로그인하는 데 사용할 수 있습니다.",
  "orySettings.lookupSecretTitle": "백업 복구 코드 (두 번째 인증 요소)",
  "orySettings.lookupSecretDescription":
    "복구 코드는 2FA의 안전한 백업으로, 2FA 장치를 분실했을 때 계정에 다시 접근할 수 있도록 해줍니다.",
  "orySettings.lookupSecretEnable": "활성화",
  "orySettings.webauthnTitle": "하드웨어 토큰 관리",
  "orySettings.webauthnDescription": "하드웨어 토큰 설정 관리",
  "orySettings.webauthnNameLabel": "보안 키 이름",
  "orySettings.webauthnAdd": "보안 키 추가",
  "orySettings.webauthnInfo":
    "하드웨어 토큰은 두 번째 인증 요소 또는 패스키와 함께 첫 번째 인증 요소로 사용됩니다.",
  "orySettings.totpTitle": "인증 앱",
  "orySettings.totpDescription":
    "계정에 TOTP 인증 앱을 추가하여 계정 보안을 강화하세요. 인기 있는 인증 앱은 LastPass와 Google Authenticator입니다.",
  "orySettings.totpSecretLabel": "인증 비밀번호",
  "orySettings.totpCodeLabel": "코드 확인",
  "orySettings.totpInfoNotLinked":
    "활성화하려면 인증 앱으로 QR 코드를 스캔하고 코드를 입력하세요.",
  "orySettings.save": "저장",
  "oryCard.loginTitle": "로그인",
  "oryCard.loginSubtitle": "{parts}로 로그인",
  "oryCard.registrationTitle": "계정 등록",
  "oryCard.registrationSubtitle": "{parts}로 가입",
  "oryCard.passwordPart": "{identifierLabel}과 비밀번호",
  "oryCard.oidcPart": "소셜 제공자",
  "oryCard.identifierPart": "{identifierLabel}",
  "oryCard.or": "또는",
  "oryCard.recoveryTitle": "계정 복구",
  "oryCard.recoverySubtitle":
    "계정과 연결된 식별자를 입력하여 일회용 액세스 코드를 받으세요",
  "oryCard.verificationTitle": "계정 확인",
  "oryCard.verificationSubtitle":
    "계정과 연결된 이메일 주소를 입력하여 확인하세요",
  "oryCard.loginFooterLabel": "계정이 없으신가요?",
  "oryCard.loginFooterLink": "가입",
  "oryCard.registrationFooterLabel": "계정이 있으신가요?",
  "oryCard.registrationFooterLink": "로그인",
};

export default ko;
