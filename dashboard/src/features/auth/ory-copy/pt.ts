import type { OryCopy } from "./keys";

// Ory Elements' own `pt` catalog entries, copied verbatim (see ./keys.ts).
const pt: OryCopy = {
  "orySettings.profileTitle": "Definições do perfil",
  "orySettings.profileDescription": "Atualizar as informações do seu perfil",
  "orySettings.passwordTitle": "Alterar palavra-passe",
  "orySettings.passwordDescription": "Modificar a sua palavra-passe",
  "orySettings.passwordLabel": "Palavra-passe",
  "orySettings.oidcTitle": "Contas associadas",
  "orySettings.oidcDescription":
    "Associe um fornecedor de início de sessão social à sua conta.",
  "orySettings.oidcInfo":
    "As contas associadas destes fornecedores podem ser usadas para iniciar sessão na sua conta.",
  "orySettings.lookupSecretTitle":
    "Códigos de recuperação de backup (segundo fator)",
  "orySettings.lookupSecretDescription":
    "Os códigos de recuperação são uma cópia de segurança segura para 2FA, permitindo-lhe recuperar o acesso se perder o dispositivo 2FA.",
  "orySettings.lookupSecretEnable":
    "Gerar novos códigos de recuperação de backup",
  "orySettings.webauthnTitle": "Gerir tokens de hardware",
  "orySettings.webauthnDescription":
    "Gerir as definições dos seus tokens de hardware",
  "orySettings.webauthnNameLabel": "Nome da chave de segurança",
  "orySettings.webauthnAdd": "Adicionar chave de segurança",
  "orySettings.webauthnInfo":
    "Os tokens de hardware são usados para autenticação de dois fatores ou como primeiro fator com passkeys.",
  "orySettings.totpTitle": "Aplicação autenticadora",
  "orySettings.totpDescription":
    "Adicione um aplicativo autenticador TOTP à sua conta para melhorar a segurança da sua conta. Aplicativos autenticadores populares são LastPass e Google Authenticator.",
  "orySettings.totpSecretLabel":
    "Este é o segredo da sua aplicação autenticadora. Utilize-o se não conseguir ler o código QR.",
  "orySettings.totpCodeLabel": "Verificar código",
  "orySettings.totpInfoNotLinked":
    "Para ativar, escaneie o código QR com seu autenticador e insira o código.",
  "orySettings.save": "Salvar",
  "oryCard.loginTitle": "Entrar",
  "oryCard.loginSubtitle": "Entre com {parts}",
  "oryCard.registrationTitle": "Registar uma conta",
  "oryCard.registrationSubtitle": "Registe-se com {parts}",
  "oryCard.passwordPart": "o seu {identifierLabel} e a palavra-passe",
  "oryCard.oidcPart": "um fornecedor social",
  "oryCard.identifierPart": "o seu {identifierLabel}",
  "oryCard.or": "ou",
  "oryCard.recoveryTitle": "Recuperar conta",
  "oryCard.recoverySubtitle":
    "Digite o identificador associado à sua conta para receber um código de acesso único",
  "oryCard.verificationTitle": "Verifique a sua conta",
  "oryCard.verificationSubtitle":
    "Introduza o endereço de email associado à sua conta para o verificar",
  "oryCard.loginFooterLabel": "Não tem uma conta?",
  "oryCard.loginFooterLink": "Registar",
  "oryCard.registrationFooterLabel": "Já tem uma conta?",
  "oryCard.registrationFooterLink": "Entrar",
};

export default pt;
