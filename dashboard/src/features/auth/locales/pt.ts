import type { TranslationEntry } from "@/i18n";

const ptAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "Bem-vindo de volta",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "Entre na sua conta",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "Crie sua conta",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "Insira seus dados para começar",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "Redefina sua senha",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message:
      "Insira o e-mail da sua conta para receber um código de acesso de uso único",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "Defina uma nova senha",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "Verifique seu e-mail",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message: "Insira o endereço de e-mail associado à sua conta para continuar",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.verificationRequiredSubtitle": {
    message: "Verifique seu endereço de e-mail para continuar usando o bex",
    description:
      "Verification page hero subtitle for a signed-in session whose email is not verified yet (ADR075 D8 verification wall)",
  },
  "auth.verificationWrongAccount": {
    message: "Não é a sua conta?",
    description:
      "Verification wall: lead-in before the sign-out link, shown only to a signed-in session",
  },
  "auth.verificationSignOut": {
    message: "Sair",
    description:
      "Verification wall: sign-out link, shown only to a signed-in session",
  },
  "auth.settingsTitle": {
    message: "Configurações",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "Gerencie sua identidade, integrações, acesso e segurança.",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "Seções de configurações",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "Conta",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message: "Atualize seu perfil, sua senha e a autenticação em duas etapas.",
    description: "Settings page account section description",
  },
  "auth.appearanceSection": {
    message: "Aparência",
    description:
      "Account settings (w3/m528): Render's Appearance section heading and its rail link; a log explorer's Options > Theme Settings opens it",
  },
  "auth.appearanceSectionSubtitle": {
    message: "Escolha a aparência do painel.",
    description: "Account settings (w3/m528): Appearance section description",
  },
  "auth.dashboardTheme": {
    message: "Tema do painel",
    description:
      "Account settings > Appearance (w3/m528): Render's Dashboard Theme setting, its card title and the select's accessible name",
  },
  "auth.dashboardThemeDescription": {
    message:
      "O tema do painel neste navegador. O Tema do menu da conta muda a mesma configuração.",
    description:
      "Account settings > Appearance (w3/m528): the Dashboard Theme card's description; the setting is the account menu's Theme, kept in this browser",
  },
  "auth.dashboardThemeSystem": {
    message: "Sistema (padrão)",
    description:
      "Account settings > Appearance (w3/m528): the System theme option, marked as the default as Render marks it",
  },
  "auth.integrationsSection": {
    message: "Integrações",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "Conecte o controle de código-fonte.",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "Credenciais de acesso",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "Gerencie o acesso programático e as identidades SSH.",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "Segurança",
    description:
      "Settings page section heading grouping the account's security cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message:
      "Controles de segurança da conta: agentes conectados e sessões ativas.",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "Zona de perigo",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "Remova sua conta permanentemente e revogue todos os acessos.",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "Excluir conta",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "Isso exclui permanentemente sua identidade, suas credenciais e os workspaces dos quais você é o único membro. Os workspaces compartilhados continuam disponíveis para os outros administradores.",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "Estes workspaces serão excluídos",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "Você sairá destes workspaces",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "Resolva primeiro a propriedade do workspace",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "Cada workspace abaixo tem outros membros, mas nenhum outro administrador. Promova outro membro, remova os outros membros ou exclua o workspace antes de excluir sua conta.",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "Abrir configurações",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "Digite {phrase} abaixo para confirmar.",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "Excluir minha conta",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "Não foi possível iniciar a exclusão da conta",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message: "Não foi possível iniciar a exclusão da conta. Tente novamente.",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "Prévia da exclusão indisponível",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "Não foi possível determinar com segurança o que acontecerá com seus workspaces. Nada foi excluído.",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "Tentar novamente",
    description: "Account deletion preview retry button",
  },
  "auth.errorTitle": {
    message: "Algo deu errado",
    description:
      "Title of /auth/error, where Kratos sends a self-service error (w5/161)",
  },
  "auth.errorSubtitle": {
    message: "Não foi possível concluir essa etapa.",
    description: "Subtitle of /auth/error",
  },
  "auth.errorGeneric": {
    message:
      "A etapa de login expirou ou não pôde ser concluída. Comece novamente pela página de login.",
    description:
      "/auth/error body when Kratos's error has no readable reason or the id is unknown",
  },
  "auth.errorBackToSignIn": {
    message: "Voltar ao login",
    description: "/auth/error button to the sign-in page",
  },
  "auth.errorFlowExpired": {
    message:
      "Essa etapa de login demorou demais e expirou. Comece novamente pela página de login.",
    description: "/auth/error message for Kratos self_service_flow_expired",
  },
  "auth.errorCsrf": {
    message:
      "A verificação de segurança do seu navegador não coincidiu — geralmente por causa de uma segunda aba ou de cookies apagados. Comece novamente pela página de login.",
    description: "/auth/error message for Kratos security_csrf_violation",
  },
  "auth.errorIdentityMismatch": {
    message:
      "Essa etapa pertence a uma conta diferente da que está conectada. Saia e comece novamente.",
    description: "/auth/error message for Kratos security_identity_mismatch",
  },
  "auth.errorReturnToForbidden": {
    message: "O endereço para retornar após o login não é permitido.",
    description:
      "/auth/error message for Kratos self_service_flow_return_to_forbidden",
  },
  "auth.accountDeletedTitle": {
    message: "Exclusão da conta iniciada",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "Você saiu com segurança.",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "Seus workspaces e credenciais estão sendo removidos. Você pode fechar esta página; nenhuma outra ação é necessária.",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "Voltar ao bex",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "Saindo...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "Encerrando sua sessão, só um instante.",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "Você saiu",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "Redirecionando para o login…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "Falha ao sair",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "Não foi possível encerrar sua sessão. Você ainda pode estar conectado — tente novamente.",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "Tentar novamente",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "Sair?",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "Tem certeza de que deseja encerrar sua sessão?",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "Sair",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "Cancelar",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "Sair",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "Seguro por padrão",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "As sessões são gerenciadas pelo Ory Kratos — uma infraestrutura de identidade testada em batalha, não um sistema de autenticação feito do zero.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "Um painel para todos os serviços",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message:
      "Faça deploy, monitore e gerencie tudo o que roda no bex em um só lugar.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "Construído de forma aberta",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "O bex é a alternativa open source ao Render.",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "Autorizar acesso",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} está pedindo para agir em seu nome no bex.",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "App de terceiros não verificado",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "ID do cliente",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "Destino do código de autorização",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "Sem redirecionamento (fluxo de dispositivo)",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "Ele está solicitando",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "Confirmar quem você é",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "Continuar conectado sem pedir novamente (refresh token)",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "Ler seu perfil básico",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "Ler seu endereço de e-mail",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message: "Ler recursos comuns do workspace (serviços, logs, métricas)",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message:
      "Alterar recursos do workspace (deploy, reiniciar, criar, excluir, faturamento)",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message:
      "Ler segredos e strings de conexão (variáveis de ambiente, URLs de banco de dados, arquivos)",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeCloudAgents": {
    message:
      "Executar agentes de programação na nuvem em seu nome (inicia tarefas que consomem computação e crédito de modelo, e podem enviar uma branch)",
    description: "Human description of the bex.cloud_agents OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message:
      "Acesso total ao plano de controle (compatibilidade legada com clientes da plataforma)",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "Onde o token funciona",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "A aprovação é lembrada por uma hora — depois disso, este agente pergunta novamente.",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "Aprovar",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "Negar",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "Algo deu errado ao concluir isso. Tente novamente.",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "Autorização expirada",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message: "Inicie a conexão novamente pelo app que enviou você até aqui.",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "Solicitação de autorização inválida",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "A solicitação deste app não tem parâmetros de segurança obrigatórios. Entre em contato com o desenvolvedor do app.",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "Este app solicitou acesso à API sem especificar do que precisa. Ele deve solicitar pelo menos um destes: {scopes}.",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "Autorização indisponível",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message: "Isso deve se resolver em breve — tente novamente daqui a pouco.",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "Conectado com a conta errada",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "A sessão deste navegador pertence a uma conta diferente da que iniciou esta autorização. Saia e tente novamente.",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "Dispositivo conectado",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "Conectar um dispositivo",
    description: "Device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message: "Um dispositivo em que você está entrando quer acessar sua conta.",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "Autorizar este dispositivo",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message:
      "Confirme se o código abaixo é igual ao exibido no seu dispositivo:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message: "Autorize somente se você mesmo iniciou este login.",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "Cancelar",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "Autorizar dispositivo",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "Código do dispositivo expirado",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "Esta página precisa de um código de dispositivo, ou o código expirou. Inicie o login novamente pelo seu dispositivo.",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "Autorização de dispositivo indisponível",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message: "Isso deve se resolver em breve — tente novamente daqui a pouco.",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "Autorização recusada",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "Não foi possível verificar esta solicitação de dispositivo. Inicie o login novamente pelo seu dispositivo.",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "A autorização no navegador foi concluída.",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message: "Volte ao app em que você está entrando para continuar.",
    description: "Device authorization success page hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "Aguardando a autorização no navegador…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "Autorização concluída. Você está conectado.",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "Você pode fechar esta aba.",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "Abrir o painel",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
  "auth.profileEmailLabel": {
    message: "E-mail",
    description:
      "Account settings profile form: the email field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileEmailPlaceholder": {
    message: "Insira seu e-mail",
    description: "Account settings profile form: the email field placeholder",
  },
  "auth.profileNameLabel": {
    message: "Nome",
    description:
      "Account settings profile form: the name field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileNamePlaceholder": {
    message: "Insira seu nome",
    description: "Account settings profile form: the name field placeholder",
  },
  "auth.inputPlaceholder": {
    message: "Insira {placeholder}",
    description:
      "Auth forms: the placeholder of a field with no wording of its own, e.g. the password; {placeholder} is the field's label",
  },
  "auth.showPassword": {
    message: "Mostrar senha",
    description:
      "Accessible name of the password field toggle while the password is hidden",
  },
  "auth.hidePassword": {
    message: "Ocultar senha",
    description:
      "Accessible name of the password field toggle while the password is shown",
  },
  "auth.ssoMoreOptions": {
    message: "Mais opções",
    description:
      "Login and sign-up: reveals the GitLab and Bitbucket sign-in buttons, as Render's More options does",
  },
  "auth.ssoContinueWith": {
    message: "Ou continue com",
    description:
      "Login and sign-up: the divider between the email form and the social sign-in buttons below it, as Render's Or continue with",
  },
  "auth.settingsGuardEmailChangeRefused": {
    message:
      "Você não pode alterar seu endereço de e-mail enquanto pertencer a um workspace que exige login com Google. Saia primeiro destes workspaces: {workspaces}.",
    description:
      "Account settings refusal of an email change (Kratos message 4100001) while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465)",
  },
  "auth.settingsGuardUnlinkRefused": {
    message:
      "Você não pode desconectar sua conta do Google enquanto pertencer a um workspace que exige login com Google. Saia primeiro destes workspaces: {workspaces}.",
    description:
      "Account settings refusal of disconnecting the Google account, or an unlink that does not name its provider (Kratos message 4100002), while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465, w7/085)",
  },
};

export default ptAuth;
