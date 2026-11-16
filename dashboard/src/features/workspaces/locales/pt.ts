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
    message: "$349,30/mês",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
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
    message: "Condições personalizadas",
    description: "Workspace plan card billing label",
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
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "Alterar plano",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "Escolha um novo plano para este workspace. Sem etapa de pagamento — o plano muda imediatamente.",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "Cancelar",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "Alterar plano",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "Não foi possível alterar o plano",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "Plano alterado para {plan}",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "Não foi possível alterar o plano do workspace",
    description: "Fallback toast/inline message on a failed plan change",
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
  "workspaces.twoFactorStatusRequired": {
    message: "Exigida",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "Não exigida",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
    message:
      "Somente administradores do workspace podem alterar esta configuração.",
    description: "Hint shown to a non-admin in place of the switch",
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
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "Mudar para outro workspace",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "Escolha um workspace",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "Nenhum outro workspace",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
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
};

export default ptWorkspaces;
