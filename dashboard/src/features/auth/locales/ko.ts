import type { TranslationEntry } from "@/i18n";

const koAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "다시 오신 것을 환영합니다",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "계정에 로그인하세요",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "계정 만들기",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "시작하려면 정보를 입력하세요",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "비밀번호 재설정",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message: "계정의 이메일을 입력하면 일회용 액세스 코드를 받을 수 있습니다",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "새 비밀번호 설정",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "이메일 인증",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message: "계속하려면 계정에 연결된 이메일 주소를 입력하세요",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.verificationRequiredSubtitle": {
    message: "bex를 계속 사용하려면 이메일 주소를 인증하세요",
    description:
      "Verification page hero subtitle for a signed-in session whose email is not verified yet (ADR075 D8 verification wall)",
  },
  "auth.verificationWrongAccount": {
    message: "내 계정이 아닌가요?",
    description:
      "Verification wall: lead-in before the sign-out link, shown only to a signed-in session",
  },
  "auth.verificationSignOut": {
    message: "로그아웃",
    description:
      "Verification wall: sign-out link, shown only to a signed-in session",
  },
  "auth.settingsTitle": {
    message: "설정",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "ID, 연동, 접근 권한, 보안을 관리하세요.",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "설정 섹션",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "계정",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message: "프로필, 비밀번호, 2단계 인증을 업데이트하세요.",
    description: "Settings page account section description",
  },
  "auth.appearanceSection": {
    message: "모양",
    description:
      "Account settings (w3/m528): Render's Appearance section heading and its rail link; a log explorer's Options > Theme Settings opens it",
  },
  "auth.appearanceSectionSubtitle": {
    message: "대시보드의 모양을 선택하세요. 선택한 설정은 계정에 저장됩니다.",
    description: "Account settings (w3/m528): Appearance section description",
  },
  "auth.dashboardTheme": {
    message: "대시보드 테마",
    description:
      "Account settings > Appearance (w3/m528, w3/m531): Render's Dashboard Theme setting, saved on the account; its row label and the select's accessible name",
  },
  "auth.dashboardThemeSystem": {
    message: "시스템(기본값)",
    description:
      "Account settings > Appearance (w3/m528): the System theme option, marked as the default as Render marks it",
  },
  "auth.logExplorerTheme": {
    message: "로그 탐색기 테마",
    description:
      "Account settings > Appearance (w3/m531): Render's Log Explorer Theme setting, its row label and the select's accessible name",
  },
  "auth.logExplorerThemeMatch": {
    message: "대시보드와 같게(기본값)",
    description:
      "Account settings > Appearance (w3/m531): the Log Explorer Theme option that follows the dashboard's theme, marked as the default as Render marks it",
  },
  "auth.highContrastMode": {
    message: "고대비 모드",
    description:
      "Account settings > Appearance (w3/m531): Render's High Contrast Mode setting, its row label and the switch's accessible name",
  },
  "auth.highContrastModeDescription": {
    message: "상호작용 요소를 더 잘 보이게 합니다.",
    description:
      "Account settings > Appearance (w3/m531): High Contrast Mode's description, Render's words",
  },
  "auth.highContrastEnabled": {
    message: "사용",
    description:
      "Account settings > Appearance (w3/m531): the text beside High Contrast Mode's switch while it is on",
  },
  "auth.highContrastDisabled": {
    message: "사용 안 함",
    description:
      "Account settings > Appearance (w3/m531): the text beside High Contrast Mode's switch while it is off",
  },
  "auth.appearanceEdit": {
    message: "편집",
    description:
      "Account settings > Appearance (w3/097): the ghost button under Dashboard Theme's and Log Explorer Theme's selects that opens the row for editing, Render's words",
  },
  "auth.appearanceCancel": {
    message: "취소",
    description:
      "Account settings > Appearance (w3/097): the button that ends a theme row's edit unsaved and puts the saved theme back, Render's words",
  },
  "auth.appearanceSaveChanges": {
    message: "변경 사항 저장",
    description:
      "Account settings > Appearance (w3/097): the button that writes a theme row's choice to the account, Render's words",
  },
  "auth.appearanceSaving": {
    message: "저장하는 중",
    description:
      "Account settings > Appearance (w3/097): the save button's text while the account is answering, Render's words",
  },
  "auth.integrationsSection": {
    message: "연동",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "소스 제어를 연결하세요.",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "접근 자격 증명",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "프로그래밍 방식 접근과 SSH ID를 관리하세요.",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "보안",
    description:
      "Settings page section heading grouping the account's security cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message: "계정 보안 제어: 연결된 에이전트와 활성 세션.",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "위험 구역",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "계정을 영구적으로 삭제하고 모든 접근 권한을 취소합니다.",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "계정 삭제",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "ID, 자격 증명, 단독 멤버인 워크스페이스가 영구적으로 삭제됩니다. 공유 워크스페이스는 다른 관리자가 계속 사용할 수 있습니다.",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "다음 워크스페이스가 삭제됩니다",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "다음 워크스페이스에서 나가게 됩니다",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "먼저 워크스페이스 소유권을 정리하세요",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "아래 워크스페이스에는 다른 멤버가 있지만 다른 관리자는 없습니다. 계정을 삭제하기 전에 다른 멤버를 관리자로 승격하거나, 다른 멤버를 제거하거나, 워크스페이스를 삭제하세요.",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "설정 열기",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "확인하려면 아래에 {phrase} 를 입력하세요.",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "내 계정 삭제",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "계정 삭제를 시작하지 못했습니다",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message: "계정 삭제를 시작하지 못했습니다. 다시 시도하세요.",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "삭제 미리보기를 사용할 수 없습니다",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "워크스페이스에 어떤 일이 일어날지 안전하게 확인하지 못했습니다. 삭제된 항목은 없습니다.",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "다시 시도",
    description: "Account deletion preview retry button",
  },
  "auth.errorTitle": {
    message: "문제가 발생했습니다",
    description:
      "Title of /auth/error, where Kratos sends a self-service error (w5/161)",
  },
  "auth.errorSubtitle": {
    message: "해당 단계를 완료하지 못했습니다.",
    description: "Subtitle of /auth/error",
  },
  "auth.errorGeneric": {
    message:
      "로그인 단계가 만료되었거나 완료되지 못했습니다. 로그인 페이지에서 다시 시작하세요.",
    description:
      "/auth/error body when Kratos's error has no readable reason or the id is unknown",
  },
  "auth.errorBackToSignIn": {
    message: "로그인으로 돌아가기",
    description: "/auth/error button to the sign-in page",
  },
  "auth.errorFlowExpired": {
    message:
      "로그인 단계가 너무 오래 걸려 만료되었습니다. 로그인 페이지에서 다시 시작하세요.",
    description: "/auth/error message for Kratos self_service_flow_expired",
  },
  "auth.errorCsrf": {
    message:
      "브라우저의 보안 검사가 일치하지 않습니다. 보통 탭을 여러 개 열었거나 쿠키를 삭제했을 때 발생합니다. 로그인 페이지에서 다시 시작하세요.",
    description: "/auth/error message for Kratos security_csrf_violation",
  },
  "auth.errorIdentityMismatch": {
    message:
      "해당 단계는 현재 로그인한 계정이 아닌 다른 계정의 것입니다. 로그아웃한 후 다시 시작하세요.",
    description: "/auth/error message for Kratos security_identity_mismatch",
  },
  "auth.errorReturnToForbidden": {
    message: "로그인 후 돌아갈 주소가 허용되지 않습니다.",
    description:
      "/auth/error message for Kratos self_service_flow_return_to_forbidden",
  },
  "auth.accountDeletedTitle": {
    message: "계정 삭제가 시작되었습니다",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "안전하게 로그아웃되었습니다.",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "워크스페이스와 자격 증명을 제거하는 중입니다. 이 페이지를 닫아도 되며, 추가로 필요한 작업은 없습니다.",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "bex로 돌아가기",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "로그아웃하는 중...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "세션을 종료하는 중입니다. 잠시만 기다리세요.",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "로그아웃되었습니다",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "로그인 페이지로 이동하는 중…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "로그아웃 실패",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "세션을 종료하지 못했습니다. 아직 로그인된 상태일 수 있으니 다시 시도하세요.",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "다시 시도",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "로그아웃하시겠습니까?",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "세션을 종료하시겠습니까?",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "로그아웃",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "취소",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "로그아웃",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "기본적으로 안전합니다",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "세션은 직접 만든 인증 시스템이 아니라 검증된 ID 인프라인 Ory Kratos가 관리합니다.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "모든 서비스를 하나의 대시보드에서",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message: "bex에서 실행되는 모든 것을 한곳에서 배포, 모니터링, 관리하세요.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "투명하게 공개된 개발",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex는 오픈 소스 Render 대안입니다.",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "접근 승인",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message:
      "{client} 앱이 bex에서 회원님을 대신하여 작업하도록 요청하고 있습니다.",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "확인되지 않은 서드파티 앱",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "클라이언트 ID",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "인증 코드 전달 대상",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "리디렉션 없음 (디바이스 흐름)",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "요청하는 권한",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "회원님이 누구인지 확인",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "다시 묻지 않고 연결 유지 (리프레시 토큰)",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "기본 프로필 읽기",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "이메일 주소 읽기",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message: "일반 워크스페이스 리소스 읽기 (서비스, 로그, 메트릭)",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message: "워크스페이스 리소스 변경 (배포, 다시 시작, 생성, 삭제, 결제)",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message: "시크릿과 연결 문자열 읽기 (환경 변수, 데이터베이스 URL, 파일)",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeCloudAgents": {
    message:
      "회원님을 대신하여 클라우드 코딩 에이전트 실행 (컴퓨팅과 모델 크레딧을 사용하는 작업을 시작하고 브랜치를 푸시할 수 있음)",
    description: "Human description of the bex.cloud_agents OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message: "전체 컨트롤 플레인 접근 (레거시 platform-client 호환)",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "토큰이 작동하는 위치",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "승인은 1시간 동안 기억되며, 그 이후에는 이 에이전트가 다시 요청합니다.",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "승인",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "거부",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "작업을 완료하는 중 문제가 발생했습니다. 다시 시도하세요.",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "승인이 만료되었습니다",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message: "이곳으로 연결한 앱에서 연결을 다시 시작하세요.",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "잘못된 승인 요청",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "이 앱의 요청에 필수 보안 매개변수가 없습니다. 앱 개발자에게 문의하세요.",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "이 앱은 필요한 항목을 지정하지 않고 API 접근을 요청했습니다. 다음 중 하나 이상을 요청해야 합니다: {scopes}.",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "승인을 진행할 수 없습니다",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message: "곧 해결될 것입니다. 잠시 후 다시 시도하세요.",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "잘못된 계정으로 로그인되어 있습니다",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "이 브라우저의 세션이 이 승인을 시작한 계정과 다른 계정에 속해 있습니다. 로그아웃한 후 다시 시도하세요.",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "디바이스가 연결되었습니다",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "디바이스 연결",
    description: "Device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message: "로그인 중인 디바이스가 계정 접근을 요청하고 있습니다.",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "이 디바이스 승인",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message: "아래 코드가 디바이스에 표시된 코드와 일치하는지 확인하세요:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message: "직접 이 로그인을 시작한 경우에만 승인하세요.",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "취소",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "디바이스 승인",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "디바이스 코드가 만료되었습니다",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "이 페이지에는 디바이스 코드가 필요하거나 코드가 만료되었습니다. 디바이스에서 로그인을 다시 시작하세요.",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "디바이스 승인을 진행할 수 없습니다",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message: "곧 해결될 것입니다. 잠시 후 다시 시도하세요.",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "승인이 거부되었습니다",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "이 디바이스 요청을 확인할 수 없습니다. 디바이스에서 로그인을 다시 시작하세요.",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "브라우저 승인이 완료되었습니다.",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message: "계속하려면 로그인 중인 앱으로 돌아가세요.",
    description: "Device authorization success page hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "브라우저 승인을 기다리는 중…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "승인이 완료되었습니다. 로그인되었습니다.",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "이 탭을 닫아도 됩니다.",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "대시보드 열기",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
  "auth.profileEmailLabel": {
    message: "이메일",
    description:
      "Account settings profile form: the email field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileEmailPlaceholder": {
    message: "이메일 입력",
    description: "Account settings profile form: the email field placeholder",
  },
  "auth.profileNameLabel": {
    message: "이름",
    description:
      "Account settings profile form: the name field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileNamePlaceholder": {
    message: "이름 입력",
    description: "Account settings profile form: the name field placeholder",
  },
  "auth.inputPlaceholder": {
    message: "{placeholder} 입력",
    description:
      "Auth forms: the placeholder of a field with no wording of its own, e.g. the password; {placeholder} is the field's label",
  },
  "auth.showPassword": {
    message: "비밀번호 표시",
    description:
      "Accessible name of the password field toggle while the password is hidden",
  },
  "auth.hidePassword": {
    message: "비밀번호 숨기기",
    description:
      "Accessible name of the password field toggle while the password is shown",
  },
  "auth.ssoMoreOptions": {
    message: "더 많은 옵션",
    description:
      "Login and sign-up: reveals the GitLab and Bitbucket sign-in buttons, as Render's More options does",
  },
  "auth.ssoContinueWith": {
    message: "또는 다음으로 계속",
    description:
      "Login and sign-up: the divider between the email form and the social sign-in buttons below it, as Render's Or continue with",
  },
  "auth.settingsGuardEmailChangeRefused": {
    message:
      "Google 로그인을 요구하는 워크스페이스에 속해 있는 동안에는 이메일 주소를 변경할 수 없습니다. 먼저 다음 워크스페이스에서 나가세요: {workspaces}.",
    description:
      "Account settings refusal of an email change (Kratos message 4100001) while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465)",
  },
  "auth.settingsGuardUnlinkRefused": {
    message:
      "Google 로그인을 요구하는 워크스페이스에 속해 있는 동안에는 Google 계정의 연결을 해제할 수 없습니다. 먼저 다음 워크스페이스에서 나가세요: {workspaces}.",
    description:
      "Account settings refusal of disconnecting the Google account, or an unlink that does not name its provider (Kratos message 4100002), while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465, w7/085)",
  },
};

export default koAuth;
