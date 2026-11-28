import type { TranslationEntry } from "@/i18n";

const ptWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "ID do workspace copiado",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "Não foi possível copiar o ID do workspace",
    description: "Toast when copying the workspace ID fails",
  },
  "workspaces.switcherEmpty": {
    message: "Selecione um workspace",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "Workspaces",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherBilling": {
    message: "Faturamento",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "Configurações do workspace",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ Novo workspace",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "Novo workspace",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "Plano",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "$0/mês",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "Para indivíduos que estão começando.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "1 membro",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "Até 25 serviços",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "5 workspaces Hobby por usuário",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17,50/mês",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "Para pequenas equipes que entregam juntas.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "Membros ilimitados",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "Serviços ilimitados",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349,30/mês por organização",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
  },
  "workspaces.planScaleDescription": {
    message: "Para equipes em crescimento que precisam de funções extras.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "Membros ilimitados",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "Serviços ilimitados",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "Funções extras (Colaborador, Visualizador, Faturamento)",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "Condições personalizadas por organização",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
  },
  "workspaces.planEnterpriseDescription": {
    message: "Limites e suporte personalizados.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "Limites personalizados",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "Suporte personalizado",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "Selecionar plano",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "Plano selecionado",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "O uso de serviços e datastores é cobrado separadamente por nível de recurso.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "Criar um workspace",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message:
      "Escolha os detalhes, o contato de faturamento, o plano e a forma de pagamento.",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "Detalhes do workspace",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "E-mail de faturamento",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message:
      "Os recibos e avisos de faturamento deste workspace são enviados para cá.",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message:
      "Em workspaces Hobby, o e-mail de faturamento é o e-mail da sua conta.",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "Informe um e-mail de faturamento válido.",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "Slug do workspace",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "Uma chave única para a CLI e a API. Letras minúsculas, números e hifens, de 1 a 30 caracteres.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "Somente letras minúsculas, números e hifens, de 1 a 30 caracteres, sem hífen no início nem no fim.",
    description: "Workspace slug validation error",
  },
  "workspaces.paymentTitle": {
    message: "Forma de pagamento",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "O faturamento é exclusivo de cada workspace. Esta forma de pagamento pertence somente ao novo workspace.",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "Uma forma de pagamento é obrigatória para este workspace.",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "Adicione uma forma de pagamento agora ou continue sem uma.",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message:
      "A cobrança de pagamentos está desativada nesta instalação auto-hospedada.",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "Adicionar forma de pagamento",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "Salvar forma de pagamento",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "Forma de pagamento verificada para este workspace.",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message:
      "Não foi possível verificar essa forma de pagamento. Tente novamente.",
    description: "Payment Element fallback error",
  },
  "workspaces.fieldName": {
    message: "Nome",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "ex.: Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "ex.: acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "Informe um nome de 1 a 100 caracteres, sem quebras de linha.",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "Plano",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "Não foi possível criar o workspace",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "Cancelar",
    description: "/new/workspace cancel button",
  },
  "workspaces.createCancelError": {
    message:
      "Não foi possível cancelar a criação do workspace. Tente novamente.",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
  },
  "workspaces.createSubmit": {
    message: "Criar workspace",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "{name} criado",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "Não foi possível criar o workspace",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.creationLoadError": {
    message:
      "Não foi possível carregar a configuração do workspace. Atualize a página para tentar novamente.",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
  },
  "workspaces.settingsTitle": {
    message: "Configurações do workspace",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message: "Renomeie este workspace ou revise o plano e os metadados dele.",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "Nenhum workspace selecionado.",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "Seções de configurações",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "Salvar",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "Não foi possível renomear o workspace",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "Renomeado para {name}",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "Não foi possível renomear o workspace",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "ID do workspace",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "Criado em",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "Zona de perigo",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "Isso excluirá todos os recursos e dados do seu workspace. Todos os serviços, datastores e variáveis de ambiente serão perdidos. Isso não pode ser desfeito.",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "Não foi possível excluir o workspace",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "Excluir workspace",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "{name} excluído",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "Não foi possível excluir o workspace",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "Alterar plano",
    description:
      "Plan link on Workspace Settings, the plan gates and the webhooks gate, opening Render's plan page (w3/m708)",
  },
  "workspaces.generalTitle": {
    message: "Geral",
    description: "Title of the general card on the workspace settings page",
  },
  "workspaces.ipRulesTitle": {
    message: "Regras de IP de entrada",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "Escolha quais endereços IP podem acessar todos os Web Services, Static Sites, bancos de dados Postgres e instâncias Key Value deste workspace. Estas regras se aplicam além das regras de ambiente e de serviço, então uma origem deve ser permitida em cada nível. Private Services, Background Workers e Cron Jobs não são afetados.",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "Informe um bloco CIDR (ex.: 203.0.113.0/24) e pressione Adicionar origem. Sem regras, todo o tráfego de entrada é bloqueado; 0.0.0.0/0 e ::/0 permitem todas as origens IPv4 e IPv6.",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "Sem regras: todo o tráfego de entrada para os serviços e datastores deste workspace está bloqueado. Adicione 0.0.0.0/0 e ::/0 para permitir todas as origens.",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "Aberto a todos os IPs de origem",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "As regras de IP de entrada de um workspace exigem o plano Scale ou superior. As regras já definidas continuam em vigor e podem ser redefinidas para permitir tudo.",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message: "Somente administradores do workspace podem alterar estas regras.",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "O plano deste workspace não inclui regras de IP de entrada. Faça upgrade para Scale ou superior para restringir o tráfego; você sempre pode redefinir as regras para permitir tudo.",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message:
      "Não foi possível carregar as regras de IP de entrada deste workspace.",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "Redefinir para permitir tudo",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "Novo bloco CIDR",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "Bloco CIDR da regra {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "Descrição da nova regra",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "Descrição da regra {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "Descrição (opcional)",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "Adicionar origem",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "Cancelar",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "Salvar regras",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "Informe blocos CIDR IPv4 ou IPv6 únicos e válidos.",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "Remover {cidr}",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "Mover {cidr} para cima",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "Mover {cidr} para baixo",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "Regras de IP de entrada salvas",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "Não foi possível salvar as regras de IP de entrada",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message:
      "Não foi possível salvar as regras de IP de entrada. Tente novamente.",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "Autenticação em duas etapas",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "Os membros sem autenticação em duas etapas (2FA) não podem acessar os recursos nem as configurações deste workspace até ativá-la. As chaves de API e as chaves SSH deles também ficam bloqueadas até lá.",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "Exigir autenticação em duas etapas (2FA)",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.loginRequirementStatusRequired": {
    message: "Exigida",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "Não exigida",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.loginRequirementAdminOnly": {
    message:
      "Somente administradores do workspace podem alterar esta configuração.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "Exigir autenticação em duas etapas requer o plano Pro ou superior. Um workspace que já a exige continua exigindo até que um administrador a desative.",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "A autenticação em duas etapas agora é exigida em {name}",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "A autenticação em duas etapas não é mais exigida em {name}",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "Não foi possível alterar a configuração de duas etapas",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message:
      "Não foi possível alterar a configuração de duas etapas. Tente novamente.",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "Ative a autenticação em duas etapas na sua própria conta antes de exigi-la neste workspace, ou você ficaria sem acesso.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message:
      "A exigência de autenticação em duas etapas não está disponível nesta instalação, então não pode ser ativada.",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "Configurar a autenticação em duas etapas",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} exige autenticação em duas etapas",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "Um administrador deste workspace exige que todos os membros usem a autenticação em duas etapas (2FA). Até você ativá-la, não é possível acessar os recursos nem as configurações deste workspace, e suas chaves de API e chaves SSH não podem ser usadas com ele.",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "Já ativei, verificar novamente",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message:
      "Depois de ativar a 2FA, pode levar até um minuto para o acesso voltar.",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.loginBlockedSwitchLabel": {
    message: "Mudar para outro workspace",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "Escolha um workspace",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "Nenhum outro workspace",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Login com Google",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "Os membros que não entram com a conta Google do seu e-mail do bex não podem acessar os recursos nem as configurações deste workspace. As chaves de API e as chaves SSH criadas com outro login também ficam bloqueadas.",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "Exigir login com Google",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "Exigir login com Google requer o plano Pro ou superior. Um workspace que já o exige continua exigindo até que um administrador o desative.",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "O login com Google agora é exigido em {name}",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "O login com Google não é mais exigido em {name}",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message: "Não foi possível alterar a configuração de login com Google",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message:
      "Não foi possível alterar a configuração de login com Google. Tente novamente.",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "Entre com a conta Google do seu e-mail do bex antes de exigir o login com Google neste workspace, ou você ficaria sem acesso.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message:
      "A exigência de login com Google não está disponível nesta instalação, então não pode ser ativada.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "Entrar com Google",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message: "Não foi possível iniciar o login com Google. Tente novamente.",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} exige login com Google",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "Um administrador deste workspace exige que todos os membros entrem com Google. Até você entrar com a conta Google do seu e-mail do bex, não é possível acessar os recursos nem as configurações deste workspace, e as chaves de API e chaves SSH que você criou com outro login não podem ser usadas com ele.",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "Você voltará para esta página depois de entrar.",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "Exigir autenticação em duas etapas em {workspace}?",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "Membros sem autenticação em duas etapas perdem o acesso a este workspace até ativá-la, e as chaves de API e chaves SSH deles deixam de funcionar nele.",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "Exigir login com Google em {workspace}?",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "Membros que não entraram com a conta Google do seu e-mail do bex perdem o acesso a este workspace até entrarem dessa forma, e as chaves de API e chaves SSH criadas com outro login deixam de funcionar nele.",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "Exigir",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "Quem perderia o acesso",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "Verificando quem perderia o acesso…",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message:
      "Não foi possível verificar quem perderia o acesso. Você ainda pode ativar.",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "Ninguém perderia o acesso: todos os membros atendem a este requisito, e nenhuma chave de API ou chave SSH deixaria de funcionar.",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "Cada membro listado perderá o acesso até ativar a autenticação em duas etapas. Cada chave de API e chave SSH listada deixará de funcionar até que o dono dela a ative.",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "Cada membro listado perderá o acesso até entrar com a conta Google do seu e-mail do bex. Cada chave de API e chave SSH listada deixará de funcionar; o dono dela pode criar uma nova depois de entrar com Google.",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "Perde o acesso",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "Mantém o acesso",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "Chave de API {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "Chave de API {name} em {workspace}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "Chave SSH {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message:
      "Chaves de API sem criador registrado, ou cujo criador não é mais membro, deixarão de funcionar:",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message:
      "Membros que atendem ao requisito, mas entraram de outra forma agora, serão solicitados a entrar com Google.",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.loginNoticeMembers_one": {
    message: "{count} membro",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_many": {
    message: "{count} de membros",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_other": {
    message: "{count} membros",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_one": {
    message:
      "{members} da sua equipe ainda não ativou a autenticação em duas etapas.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_many": {
    message:
      "{members} da sua equipe ainda não ativaram a autenticação em duas etapas.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message:
      "{members} da sua equipe ainda não ativaram a autenticação em duas etapas.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_one": {
    message: "{members} da sua equipe ainda não fez login com o Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_many": {
    message: "{members} da sua equipe ainda não fizeram login com o Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message: "{members} da sua equipe ainda não fizeram login com o Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_one": {
    message: "Ver membro",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_many": {
    message: "Ver membros",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "Ver membros",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "Todos os membros",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message: "{members} da sua equipe ativaram a autenticação em duas etapas!",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "{members} da sua equipe fizeram login com o Google!",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "Membros sem autenticação em duas etapas",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "Membros sem login com Google",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_one": {
    message:
      "Este membro está proibido no momento de acessar este workspace da equipe e os recursos nele.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_many": {
    message:
      "Estes membros estão proibidos no momento de acessar este workspace da equipe e os recursos nele.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message:
      "Estes membros estão proibidos no momento de acessar este workspace da equipe e os recursos nele.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_one": {
    message: "Lembrar este usuário de ativar a 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_many": {
    message: "Lembrar estes usuários de ativar a 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "Lembrar estes usuários de ativar a 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_one": {
    message: "Envie um lembrete por e-mail para este usuário.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_many": {
    message: "Envie lembretes por e-mail para estes usuários.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "Envie lembretes por e-mail para estes usuários.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_one": {
    message: "Enviar lembrete",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_many": {
    message: "Enviar lembretes",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "Enviar lembretes",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message:
      "Sentimos muito. Não conseguimos enviar lembretes para estes membros da equipe. Tentar novamente?",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message:
      "Lembretes enviados por e-mail aos membros sem autenticação em duas etapas!",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "O limite de lembretes foi atingido!",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message:
      "Parece que já enviamos lembretes hoje. Você pode enviar lembretes novamente amanhã.",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_one": {
    message: "Um lembrete para ativar a 2FA será enviado a este membro.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_many": {
    message: "Lembretes para ativar a 2FA serão enviados a estes membros.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "Lembretes para ativar a 2FA serão enviados a estes membros.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "Política de deploys sobrepostos",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "Escolha o que acontece quando o deploy de um serviço é disparado enquanto outro deploy dele está em andamento.",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "Aguardar",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "Permita que o deploy em andamento termine e depois siga direto para o deploy disparado mais recentemente.",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "Substituir",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "Cancele imediatamente o deploy em andamento e inicie o novo.",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "Salvar",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "Salvando…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "Política de deploys sobrepostos salva",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message:
      "Não foi possível salvar a política de deploys sobrepostos. Tente novamente.",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message:
      "Somente administradores do workspace podem alterar esta política.",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
  "workspaces.orgDowngradePersonRole": {
    message: "{person} ({role})",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "este workspace",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "o ambiente {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "o serviço {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message:
      "O plano deste workspace é o plano da organização. Altere-o no workspace principal da organização.",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message:
      "Incluído no plano da sua organização. O workspace principal da organização gerencia o plano e o faturamento.",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "Ir para o workspace principal",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "Ir para o faturamento do workspace principal",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "Privacidade do workspace",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message: "Escolha quem da sua organização pode entrar neste workspace.",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "Público",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message:
      "Qualquer membro da sua organização pode se adicionar a este workspace.",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "Somente por convite",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message:
      "Somente administradores do workspace podem convidar pessoas para este workspace.",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "Salvar",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "Salvando…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "Privacidade do workspace salva",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message:
      "Não foi possível salvar a privacidade do workspace. Tente novamente.",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message: "Exigido por {organization} em todos os seus workspaces.",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "Altere nas configurações da organização",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message:
      "{organization} exige autenticação em duas etapas em todos os seus workspaces",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "Um proprietário desta organização exige que todos os membros usem a autenticação em duas etapas (2FA) na organização e em todos os seus workspaces. Até você ativá-la, não é possível acessá-los, e suas chaves de API e chaves SSH não podem ser usadas com eles.",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message:
      "{organization} exige login com Google em todos os seus workspaces",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "Um proprietário desta organização exige que todos os membros entrem com Google na organização e em todos os seus workspaces. Até você entrar com a conta Google do seu e-mail do bex, não é possível acessá-los, e as chaves de API e chaves SSH que você criou com outro login não podem ser usadas com eles.",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.updatePlanPageTitle": {
    message: "Alterar plano",
    description: "Document title of the plan pages (w3/m708)",
  },
  "workspaces.updatePlanTitle": {
    message: "Alterar plano de {name}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): page title with the workspace's name",
  },
  "workspaces.updatePlanConfirmTitle": {
    message: "Alterar plano de {name}",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): page title with the workspace's name (Render capitalizes Plan here)",
  },
  "workspaces.updatePlanBack": {
    message: "Voltar",
    description:
      "Accessible label of the plan pages' Back button, and the confirmation's Back action (w3/m708)",
  },
  "workspaces.updatePlanSelectTitle": {
    message: "Selecione um plano",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the plan cards",
  },
  "workspaces.updatePlanCompare": {
    message: "Saiba mais ou compare os planos na nossa {link}.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under Select a plan; {link} is the updatePlanPricingLink link to bex's pricing page",
  },
  "workspaces.updatePlanPricingLink": {
    message: "página de preços",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): the link in updatePlanCompare",
  },
  "workspaces.updatePlanPaymentTitle": {
    message: "Forma de pagamento",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the payment method section",
  },
  "workspaces.updatePlanCardRequired": {
    message:
      "Este plano de workspace exige um cartão para serviços pagos e computação.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): notice while a plan other than Hobby is selected",
  },
  "workspaces.updatePlanHowBillingWorks": {
    message: "Como funciona a cobrança",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): link beside updatePlanCardRequired to bex's billing docs",
  },
  "workspaces.updatePlanWorkspaceBillingTitle": {
    message: "Cobrança do workspace",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading beside the card on file",
  },
  "workspaces.updatePlanWorkspaceBillingBody": {
    message:
      "A cobrança é separada para cada workspace. Seu cartão será cobrado pelas taxas de serviços pagos ou computação neste workspace.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under updatePlanWorkspaceBillingTitle",
  },
  "workspaces.updatePlanErrorsSummary": {
    message:
      "Há alguns erros na sua seleção. Revise e corrija-os para continuar.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): summary under the plan rules a selection breaks",
  },
  "workspaces.updatePlanErrorPrefix": {
    message: "Ocorreu um erro. {message}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): a refused change; {message} is the reason",
  },
  "workspaces.updatePlanPaymentRequired": {
    message: "Adicione um cartão válido para fazer upgrade para um plano pago.",
    description:
      "The plan pages' message when bex-api refuses a paid plan with PAYMENT_REQUIRED (w3/m708)",
  },
  "workspaces.updatePlanServerError": {
    message: "Erro no servidor ao enviar. Tente novamente",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): fallback reason when an upgrade fails without one",
  },
  "workspaces.updatePlanSubmit": {
    message: "Selecionar plano",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): submit button",
  },
  "workspaces.updatePlanCancel": {
    message: "Cancelar",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): cancel button, back to where the page was opened from",
  },
  "workspaces.updatePlanNoPermission": {
    message:
      "Sua função não pode alterar o plano deste workspace. Peça a um administrador do workspace.",
    description:
      "The plan pages' notice for a member who cannot change the plan (can_manage) (w3/m708)",
  },
  "workspaces.planCurrentBadge": {
    message: "Plano atual",
    description:
      "Badge on the workspace's own plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planNewBadge": {
    message: "Novo plano",
    description:
      "Badge on the chosen plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planContact": {
    message: "Fale conosco",
    description:
      "Enterprise card's contact action on the plan page, Render's Get in touch (w3/m708)",
  },
  "workspaces.updatePlanSummaryTitle": {
    message: "Resumo do novo plano",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the plan comparison and the card on file",
  },
  "workspaces.updatePlanConfirmDowngradeTitle": {
    message: "Confirmar downgrade",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the typed confirmation a downgrade asks for",
  },
  "workspaces.updatePlanConfirmChangesIncluding": {
    message: "Para iniciar as alterações no seu plano, incluindo:",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): lead-in above the list of what a downgrade changes",
  },
  "workspaces.updatePlanConfirmTypeAfterList": {
    message:
      "Digite {phrase} na caixa de texto e clique no botão de confirmar.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase after the list of changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmType": {
    message:
      "Para iniciar as alterações no seu plano, digite {phrase} na caixa de texto e clique no botão de confirmar.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase when the downgrade lists no changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmSubmit": {
    message: "Confirmar",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): confirm button",
  },
  "workspaces.updatePlanErrorTitle": {
    message: "Ocorreu um erro.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): title of a refused change",
  },
  "workspaces.updatePlanError": {
    message: "Erro ao atualizar o plano.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): fallback reason when the change fails without one",
  },
  "workspaces.updatePlanRemovesOrganization": {
    message: "A organização será removida.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): bold start of the change an org primary's downgrade below Scale makes (ADR115 D6)",
  },
  "workspaces.updatePlanRemovesOrganizationDetail": {
    message:
      "Os recursos de nível de organização terminam, e este workspace continua como um workspace independente.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): rest of the org removal line after updatePlanRemovesOrganization",
  },
  "workspaces.planUpdated": {
    message: "Seu workspace agora está no plano {plan}.",
    description:
      "Toast after a plan change, Render's plan-updated toast; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planUpdatedWelcome": {
    message: "Boas-vindas ao plano {plan}.",
    description:
      "Toast after a change to Scale, Render's welcome; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planComparisonCurrent": {
    message: "Plano atual",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the current plan's side of the comparison",
  },
  "workspaces.planComparisonNew": {
    message: "Novo plano",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the new plan's side of the comparison",
  },
  "workspaces.planComparisonHighlights": {
    message: "Destaques",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): label above each side's highlights (shown in capitals)",
  },
  "workspaces.planHighlightJustForMe": {
    message: "Só para mim",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's one seat",
  },
  "workspaces.planHighlightUnlimitedMembers": {
    message: "Membros da equipe ilimitados",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited seats",
  },
  "workspaces.planHighlightIncludedDomains": {
    message:
      "{included} domínios personalizados incluídos, depois {price}/domínio",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): custom domains included before they are metered; {included} is the count, {price} bex's price per domain (ADR030 §16)",
  },
  "workspaces.planHighlightUnlimitedDomains": {
    message: "Domínios personalizados ilimitados",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): every custom domain included (Enterprise)",
  },
  "workspaces.planHighlightTwoEnvironments": {
    message: "2 ambientes por projeto",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's environments per project",
  },
  "workspaces.planHighlightUnlimitedEnvironments": {
    message: "Projetos e ambientes ilimitados",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited projects and environments",
  },
  "workspaces.planHighlightRetention": {
    message: "Janela de {days} dias para logs e métricas",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): how far back logs and metrics reach; {days} is the number of days",
  },
  "workspaces.planHighlightBasicLogs": {
    message: "Logs e métricas básicos",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's logs and metrics, without request logs",
  },
  "workspaces.planHighlightRequestLogs": {
    message: "Logs de requisições HTTP e métricas de tempo de resposta",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Pro's HTTP request logs and response time metrics",
  },
  "workspaces.planHighlightKeptBuilds": {
    message: "{builds} builds mantidos para rollback instantâneo",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): builds kept for rollback; {builds} is the count (core.KeptBuilds)",
  },
  "workspaces.planHighlightAuditLogs": {
    message: "Logs de auditoria do workspace",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): workspace audit logs (Pro and higher)",
  },
  "workspaces.planHighlightOrganization": {
    message:
      "Uma organização, com mais workspaces, funções extras e regras de IP de entrada",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): what an organization adds (Scale and higher, ADR115)",
  },
  "workspaces.planBlockerMemberLimitTitle": {
    message: "Usuários demais na equipe.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more people than the plan seats",
  },
  "workspaces.planBlockerMemberLimitOne": {
    message: "Workspaces {plan} só podem ter 1 usuário.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with one seat; {plan} is the plan's name",
  },
  "workspaces.planBlockerMemberLimitMany": {
    message: "Workspaces {plan} só podem ter até {limit} usuários.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with several seats; {plan} is the plan's name, {limit} its seats",
  },
  "workspaces.planBlockerUpdateTeamMembers": {
    message: "Atualizar membros da equipe",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' Team section",
  },
  "workspaces.planBlockerServiceLimitTitle": {
    message: "Serviços demais no workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more services than the plan allows",
  },
  "workspaces.planBlockerServiceLimit": {
    message: "Workspaces {plan} só podem ter até {limit} serviços.",
    description:
      "Render's plan rule on its plan page (w3/m708): the plan's service cap; {plan} is the plan's name, {limit} its cap",
  },
  "workspaces.planBlockerUpdateServices": {
    message: "Atualizar serviços",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the workspace's services",
  },
  "workspaces.planBlockerWorkspaceLimitTitle": {
    message: "Workspaces Hobby demais.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the caller already has the most Hobby workspaces allowed",
  },
  "workspaces.planBlockerWorkspaceLimit": {
    message:
      "Você não pode ter mais de {limit} workspaces Hobby. Faça upgrade ou exclua outro workspace Hobby.",
    description:
      "Render's plan rule on its plan page (w3/m708): the cap on a person's Hobby workspaces; {limit} is the cap",
  },
  "workspaces.planBlockerWorkspaceLimitsDocs": {
    message: "Limites de workspace",
    description:
      "Render's plan rule on its plan page (w3/m708): link to bex's docs on workspace limits",
  },
  "workspaces.planBlockerWorkspacesTitle": {
    message: "Workspaces demais.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when an org primary's downgrade leaves secondary workspaces",
  },
  "workspaces.planBlockerWorkspaces": {
    message: "{plan} não oferece suporte a mais de um workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan without an organization; {plan} is the plan's name",
  },
  "workspaces.planBlockerUpdateWorkspaces": {
    message: "Atualizar workspaces",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the organization's workspaces",
  },
  "workspaces.planBlockerRolesTitle": {
    message:
      "Não é possível fazer downgrade enquanto funções indisponíveis estiverem em uso",
    description:
      "Render's plan rule on its plan page (w3/m708): title when members or invites hold roles the plan does not offer",
  },
  "workspaces.planBlockerRolesOne": {
    message:
      "{plan} não oferece suporte à função {roles}. Para fazer downgrade do workspace, primeiro altere os membros da equipe e os convites pendentes afetados para Developer ou Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): one role the plan lacks; {plan} is the plan's name, {roles} the role",
  },
  "workspaces.planBlockerRolesMany": {
    message:
      "{plan} não oferece suporte às funções {roles}. Para fazer downgrade do workspace, primeiro altere os membros da equipe e os convites pendentes afetados para Developer ou Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): several roles the plan lacks; {plan} is the plan's name, {roles} the roles",
  },
  "workspaces.planBlockerIpRulesTitle": {
    message:
      "As listas de IPs permitidos devem ser removidas antes do downgrade.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when inbound IP rules stand in the way (Render's words for them on its confirmation)",
  },
  "workspaces.planBlockerIpRules": {
    message:
      "Remova todas as configurações de listas de IPs permitidos do workspace, dos ambientes e dos serviços antes de alterar o plano.",
    description:
      "Render's plan rule on its plan page (w3/m708): what to remove for planBlockerIpRulesTitle",
  },
  "workspaces.planBlockerUpdateIpRules": {
    message: "Atualizar regras de IP de entrada",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' inbound IP rules",
  },
  "workspaces.planBlockerOtherTitle": {
    message: "Este plano ainda não pode ser selecionado.",
    description:
      "Render's plan rule on its plan page (w3/m708): title of a refusal kind this dashboard does not know yet",
  },
  "workspaces.planBlockerOther": {
    message: "Este workspace ainda não atende aos limites do plano {plan}.",
    description:
      "Render's plan rule on its plan page (w3/m708): line under planBlockerOtherTitle; {plan} is the plan's name",
  },
};

export default ptWorkspaces;
