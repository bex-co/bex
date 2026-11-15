import type { TranslationEntry } from "@/i18n";

const ptSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "Novo grupo de sandbox",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "Um grupo de sandbox limita quantas de suas sandboxes podem estar ativas ao mesmo tempo. Os nomes dos grupos devem ser exclusivos no seu workspace.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Nome",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Região",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Cancelar",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Criar grupo de sandbox",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message:
      "Não foi possível criar o grupo de sandbox {name}. Tente novamente.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "Já existe um grupo de sandbox chamado “{name}” neste workspace. Escolha outro nome.",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "O nome do grupo deve ter de 1 a {max} caracteres, sem espaços no início ou no fim e sem caracteres de controle.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message: "Este workspace já tem o máximo de {count} grupo de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_many": {
    message: "Este workspace já tem o máximo de {count} de grupos de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReached_other": {
    message: "Este workspace já tem o máximo de {count} grupos de sandbox.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "Este workspace atingiu o limite de grupos de sandbox.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Limite de grupos de sandbox atingido",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Grupo de sandbox criado",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} está pronto. Passe o ID dele como sandboxGroupId ao criar uma sandbox nele.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Concluir",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Grupo padrão",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Sim",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "Não",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Limite de concorrência",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Copiar ID do grupo de sandbox",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "ID do grupo de sandbox copiado",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "Não foi possível copiar o ID do grupo de sandbox",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "Carregando ambientes",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "Não foi possível carregar seus ambientes. Você ainda pode criar o grupo sem um ambiente.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message: "Esse ambiente não existe mais. Escolha outro ou nenhum.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
};

export default ptSandboxGroups;
