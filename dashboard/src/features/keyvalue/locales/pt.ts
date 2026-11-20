import type { TranslationEntry } from "@/i18n";

const ptKeyValue: Record<string, TranslationEntry> = {
  "keyvalue.resourceType": {
    message: "Key Value",
    description: "Key Value resource type used in document titles",
  },
  "keyvalue.statTotal": {
    message: "Total de stores Key Value",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "Disponível",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "Criando",
    description: "Key Value page stat card label (provisioning stores)",
  },
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "Nome",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "Status",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "Plano de computação",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "Versão",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "Criado",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "Ações",
    description: "Key Value table actions column header (screen-reader only)",
  },
  "keyvalue.statusAvailable": {
    message: "Disponível",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "Criando",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusRestarting": {
    message: "Reiniciando",
    description:
      "Key Value status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "keyvalue.statusUnavailable": {
    message: "Indisponível",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "Suspenso",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "Desconhecido",
    description: "Key Value status badge for an unrecognized status",
  },
  "keyvalue.statusReasonStatefulSetReadFailed": {
    message: "Não foi possível verificar o status do workload do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStatefulSetFailed": {
    message:
      "Não foi possível atualizar o workload do Key Value, então a alteração mais recente não foi aplicada.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCReadFailed": {
    message: "Não foi possível verificar o status do volume de armazenamento.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCResizeFailed": {
    message:
      "Não foi possível redimensionar o volume de armazenamento do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCResizeFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretFailed": {
    message: "Não foi possível salvar os detalhes de conexão do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCredentialSecretFailed": {
    message: "Não foi possível salvar as credenciais do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CredentialSecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretRecreateFailed": {
    message:
      "Não foi possível reconstruir os detalhes de conexão do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretRecreateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonServiceFailed": {
    message: "Não foi possível configurar o endereço interno do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's ServiceFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonNetworkPolicyFailed": {
    message: "Não foi possível aplicar a política de rede do Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupNetworkPolicyFailed": {
    message: "Não foi possível aplicar a política de rede do job de backup.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupNetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupCronJobFailed": {
    message: "Não foi possível atualizar o agendamento do backup diário.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupCronJobFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonTLSIssuerMissing": {
    message:
      "Não é possível ativar o acesso público: nenhum emissor de certificados está configurado.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's TLSIssuerMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateFailed": {
    message: "Não foi possível emitir o certificado para o acesso público.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateCleanupFailed": {
    message: "Não foi possível remover o certificado do acesso público.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassMissing": {
    message:
      "O volume de armazenamento não tem classe de armazenamento, então não pode crescer.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotFound": {
    message:
      "A classe de armazenamento do volume não foi encontrada, então ele não pode crescer.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotFound Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotExpandable": {
    message: "A classe de armazenamento do volume não permite que ele cresça.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotExpandable Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageBlockedByQuota": {
    message:
      "O volume de armazenamento não pode crescer: a cota de armazenamento do workspace não tem espaço para ele.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageBlockedByQuota Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceTransitionFailed": {
    message:
      "Falha ao preparar os dados armazenados para a inicialização; os dados são mantidos e será feita uma nova tentativa.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceTransitionFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceSourceUnknown": {
    message:
      "O modo de persistência dos dados armazenados é desconhecido, então o Key Value não pode iniciar até que um operador da plataforma o confirme. Nenhum dado foi alterado.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceSourceUnknown Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageShrinkRejected": {
    message:
      "O armazenamento do Key Value só cresce: o volume não pode ficar menor que o tamanho atual.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.errorTitle": {
    message: "Não foi possível carregar os stores Key Value",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message:
      "A requisição ao bex-api falhou. Verifique sua conexão e tente novamente.",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "Nenhum store Key Value ainda",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message:
      "Crie seu primeiro store Key Value gerenciado e ele aparecerá aqui.",
    description: "Key Value list empty state body",
  },
  "keyvalue.actionsMenu": {
    message: "Ações para {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "Excluir",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Excluir instância Key Value",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "Esta ação não pode ser desfeita. Tem certeza de que deseja excluir esta instância Key Value?",
    description:
      "Delete-confirmation dialog body (verbatim Render copy, docs/render-artifacts/key-value.md)",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming Render's exact sudo delete phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Excluir instância Key Value",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "Excluindo {name}…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "Não foi possível excluir {name}. Tente novamente.",
    description: "Toast when a delete request fails",
  },
  "keyvalue.createButton": {
    message: "Novo Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "Novo Key Value",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message:
      "Provisione uma instância gerenciada do Valkey (compatível com Redis).",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "Nome",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message:
      "Informe um nome de até 100 caracteres, sem quebras de linha ou caracteres de controle.",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "Plano de computação",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Versão do Valkey",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "Padrão (mais recente)",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Política de maxmemory",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "(recomendada para caches)",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message:
      "Como as chaves são removidas quando o store atinge o limite de memória.",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "Modo de persistência",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message:
      "Como os dados são persistidos em disco para sobreviver a uma reinicialização.",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "Journal + Snapshot",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "Somente snapshot",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "Desativada",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "Acesso público",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message: "Permita conexões de fora do cluster via TLS.",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "Cancelar",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Criar instância Key Value",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "Criando {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "Não foi possível criar {name}. Tente novamente.",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Limite de Key Value atingido",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "Fazer upgrade do plano",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  "keyvalue.metaTitle": {
    message: "Detalhes",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "Status",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "Plano de computação",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaConnectionLimit": {
    message: "Limite de conexões",
    description:
      "Key Value detail Details card row label: the plan's client connection limit, as Render's Key Value Instance card shows it",
  },
  "keyvalue.metaVersion": {
    message: "Versão",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "Acesso público",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "Host externo",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "Região",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "Criado",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "Sim",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.publicPending": {
    message: "Sim (endpoint externo sendo aberto: certificado pendente)",
    description:
      "Details value for Public access while the store's certificate is pending, so it does not read as reachable (w4/m357)",
  },
  "keyvalue.publicFailed": {
    message: "Sim (ainda inacessível: a emissão do certificado falhou)",
    description:
      "Details value for Public access when the store's certificate failed to issue (w4/m357)",
  },
  "keyvalue.no": {
    message: "Não",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.connTitle": {
    message: "Conexões",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "Strings de conexão deste store Key Value. Reveladas somente quando você pedir — nunca exibidas automaticamente.",
    description: "Connection-info panel card description",
  },
  "keyvalue.internalAuthOff": {
    message: "As conexões internas não exigem senha.",
    description: "Connections panel: internal auth off",
  },
  "keyvalue.internalAuthOn": {
    message: "As conexões internas exigem a senha.",
    description: "Connections panel: internal auth on",
  },
  "keyvalue.internalAuthEnable": {
    message: "Ativar autenticação interna",
    description: "Button and confirm label (Render's wording)",
  },
  "keyvalue.internalAuthConfirmTitle": {
    message: "Ativar a autenticação interna?",
    description: "Confirm dialog title",
  },
  "keyvalue.internalAuthConfirmBody": {
    message:
      "Toda conexão interna passará a exigir a senha. As conexões existentes que usam a URL sem autenticação deixarão de funcionar, e os serviços vinculados adotam a nova URL no próximo deploy; atualize-os para a URL mostrada nas informações de conexão. Isso não pode ser desfeito.",
    description: "Confirm dialog body",
  },
  "keyvalue.internalAuthEnabled": {
    message: "Autenticação interna ativada para {name}.",
    description: "Toast after enabling",
  },
  "keyvalue.internalAuthError": {
    message: "Não foi possível ativar a autenticação interna. Tente novamente.",
    description: "Toast when enabling fails",
  },
  "keyvalue.connReveal": {
    message: "Revelar informações de conexão",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "Ocultar informações de conexão",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "URL interna do Key Value",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "URL externa do Key Value",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalPending": {
    message:
      "O acesso externo está sendo ativado: um certificado publicamente confiável está sendo emitido (geralmente leva alguns minutos). A URL externa e o comando da CLI conectam quando o armazenamento aceitar conexões TLS; até lá, ele só pode ser acessado pelos seus serviços, com a URL interna. Oculte e revele novamente para atualizar.",
    description:
      "Shown under the external URL while a public store's TLS listener is still opening (w9/m521)",
  },
  "keyvalue.connExternalFailed": {
    message:
      "A URL externa e o comando da CLI ainda não conectam: a emissão do certificado publicamente confiável deste armazenamento falhou, e o bex tenta novamente de forma automática. Até que tenha êxito, o armazenamento só pode ser acessado pelos seus serviços, com a URL interna.",
    description:
      "Shown under the external URL when a public store's certificate failed to issue (w4/m357, w9/m521)",
  },
  "keyvalue.connExternalUnavailable": {
    message:
      "Não é público. Adicione uma regra de IP de entrada em Rede para ativar o acesso externo.",
    description:
      "Shown instead of the external URL when the store isn't public",
  },
  "keyvalue.connCli": {
    message: "Comando do Valkey CLI",
    description: "Connection-info field label (ready-to-run valkey-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "Não foi possível carregar as informações de conexão",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "O store pode ainda estar sendo provisionado, ou você pode não ter permissão para ver suas credenciais.",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "Copiado para a área de transferência",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Toast when clipboard copy fails",
  },
  "keyvalue.dangerDelete": {
    message: "Excluir instância Key Value",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Suspender instância Key Value",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Retomar instância Key Value",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Suspender instância Key Value",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "Tem certeza de que deseja suspender esta instância Key Value?",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "Confirme que deseja suspender {name}. As conexões ativas serão encerradas, e você pode retomá-la a qualquer momento. A retenção de dados segue o modo de persistência: Desativada perde todos os dados; Somente snapshot mantém os dados até o último snapshot; Journal + Snapshot mantém as gravações até cerca do último segundo.",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming Render's exact Key Value suspend phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.confirmCancel": {
    message: "Cancelar",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "Suspendendo {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "Não foi possível suspender {name}. Tente novamente.",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "Retomando {name}…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "Não foi possível retomar {name}. Tente novamente.",
    description: "Toast when a resume request fails",
  },
  "keyvalue.planTitle": {
    message: "Plano de computação",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "Mude para um plano de computação maior; uma instância Key Value não pode passar para um menor. A mudança reinicia o store, que fica indisponível por um ou dois minutos. Os dados sobrevivem conforme o modo de persistência; um store com persistência desativada perde todas as chaves.",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planDowngradeUnavailable": {
    message:
      "Uma instância Key Value não pode passar para um plano de computação menor. Crie uma nova instância neste plano e mova seus dados para ela.",
    description:
      "Tooltip on a disabled plan card smaller than the store's current plan: Render does not downgrade a Key Value instance (w8/249)",
  },
  "keyvalue.planPausedReason": {
    message:
      "O plano Key Value “{name}” está temporariamente indisponível; escolha {largest} ou um plano menor.",
    description:
      "A paused Key Value plan's tooltip and screen-reader reason, composed in the dashboard's language; matches bex-api's English unavailableReason (w4/552)",
  },
  "keyvalue.planPickerSave": {
    message: "Salvar",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "Cancelar",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.planPickerConfirmTitle": {
    message: "Mudar para {name}?",
    description: "Key Value plan-picker confirm dialog title",
  },
  "keyvalue.planPickerConfirmBody": {
    message:
      "O store reinicia para aplicar o novo plano de computação e fica indisponível por um ou dois minutos. As conexões caem.",
    description: "Key Value plan-picker confirm dialog body",
  },
  "keyvalue.restartConfirmDataLoss": {
    message:
      "A persistência está desativada, então a reinicialização exclui todas as chaves de {name}. Isso não pode ser desfeito.",
    description:
      "Restart confirm (plan change): a Persistence Off store loses all data on the restart (w4/m263)",
  },
  "keyvalue.restartConfirmDataLossUnknown": {
    message:
      "O modo de persistência deste store é desconhecido. Se estiver desativada, a reinicialização exclui todas as chaves de {name}. Isso não pode ser desfeito.",
    description:
      "Restart confirm when the persistence mode could not be read: the typed phrase is still required",
  },
  "keyvalue.planPickerConfirmPrice": {
    message: "O novo plano custa ${price}/mês.",
    description:
      "Plan-change confirm: the new plan's monthly price, from the API's price sheet (w4/156)",
  },
  "keyvalue.planPickerSuccess": {
    message: "Atualizando o plano para {name}…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "Não foi possível atualizar o plano. Tente novamente.",
    description: "Toast when a plan update fails",
  },
  "keyvalue.maxmemoryTitle": {
    message: "Política de maxmemory",
    description: "Key Value detail eviction-policy card title",
  },
  "keyvalue.maxmemoryDescription": {
    message:
      "Altere como as chaves são removidas quando o store atinge o limite de memória. A nova política se aplica ao store em execução, sem reinicialização.",
    description: "Key Value detail eviction-policy card description",
  },
  "keyvalue.maxmemoryLabel": {
    message: "Política de maxmemory",
    description: "Accessible label for the eviction-policy select",
  },
  "keyvalue.maxmemoryEdit": {
    message: "Editar política de maxmemory",
    description:
      "Accessible label for the maxmemory-policy edit-in-place pencil",
  },
  "keyvalue.maxmemorySuccess": {
    message: "Atualizando a política de maxmemory para {policy}…",
    description: "Toast after an eviction-policy update is accepted",
  },
  "keyvalue.maxmemoryError": {
    message:
      "Não foi possível atualizar a política de maxmemory. Tente novamente.",
    description: "Toast when an eviction-policy update fails",
  },
  "keyvalue.persistenceTitle": {
    message: "Modo de persistência",
    description: "Key Value detail persistence-mode card title",
  },
  "keyvalue.persistenceDescription": {
    message:
      "Mudar de modo reinicia o store e interrompe as conexões enquanto os dados são preparados. Alternar entre Journal + Snapshot e Somente snapshot preserva os dados. Passar para Desativada ou sair dela descarta todos os dados.",
    description: "Key Value detail persistence-mode card description",
  },
  "keyvalue.persistenceLabel": {
    message: "Modo de persistência",
    description: "Accessible label for the persistence-mode select",
  },
  "keyvalue.persistenceEdit": {
    message: "Editar modo de persistência",
    description:
      "Accessible label for the persistence-mode edit-in-place pencil",
  },
  "keyvalue.persistenceSuccess": {
    message: "Atualizando o modo de persistência para {mode}…",
    description: "Toast after a persistence-mode update is accepted",
  },
  "keyvalue.persistenceConfirmTitle": {
    message: "Excluir todos os dados e alterar a persistência?",
    description: "Destructive persistence-mode confirmation title",
  },
  "keyvalue.persistenceConfirmDescription": {
    message:
      "Passar para Desativada ou sair dela exclui todos os dados de {name} e reinicia o store. Isso não pode ser desfeito.",
    description: "Destructive persistence-mode confirmation description",
  },
  "keyvalue.persistenceConfirmAction": {
    message: "Excluir dados e alterar modo",
    description: "Destructive persistence-mode confirmation action",
  },
  "keyvalue.persistenceError": {
    message:
      "Não foi possível atualizar o modo de persistência. Tente novamente.",
    description: "Toast when a persistence-mode update fails",
  },
  "keyvalue.networkingTitle": {
    message: "Rede",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message: "Restrinja quais IPs de origem podem acessar o endpoint externo.",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "Salvar regras CIDR ativa o acesso externo para os IPs de origem correspondentes. Salvar uma lista vazia bloqueia as conexões externas. O acesso interno não muda.",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingHintRestart": {
    message:
      "Salvar regras CIDR ativa o acesso externo para os IPs de origem correspondentes. Este store nunca teve acesso externo, então o primeiro salvamento o reinicia e as conexões caem por até um minuto. Salvar uma lista vazia bloqueia as conexões externas.",
    description:
      "Networking IP allowlist helper text when enabling external access restarts the store (its first enable)",
  },
  "keyvalue.networkingRestartConfirmTitle": {
    message: "Excluir todos os dados e ativar o acesso externo?",
    description:
      "Confirmation title when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmDescription": {
    message:
      "Ativar o acesso externo reinicia {name} pela primeira vez. O modo de persistência dele é Desativada, então a reinicialização exclui todos os dados. Isso não pode ser desfeito.",
    description:
      "Confirmation description when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmAction": {
    message: "Excluir dados e salvar regras",
    description:
      "Confirmation action when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "As conexões externas estão desativadas. Salve uma ou mais regras de IP para ativar o acesso.",
    description: "External access is disabled; saving rules enables it",
  },
  "keyvalue.networkingEmpty": {
    message:
      "Nenhuma regra de IP. Salve uma lista vazia para bloquear as conexões externas.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "keyvalue.networkingAdd": {
    message: "Adicionar",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "Descrição (opcional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingCIDR": {
    message: "Novo bloco CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "keyvalue.networkingCIDRRule": {
    message: "Bloco CIDR da regra {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "keyvalue.networkingDescriptionLabel": {
    message: "Descrição da nova regra",
    description: "Accessible label for the add-row description input",
  },
  "keyvalue.networkingDescriptionRule": {
    message: "Descrição da regra {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "keyvalue.networkingRemove": {
    message: "Remover {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingMoveUp": {
    message: "Mover {cidr} para cima",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "keyvalue.networkingMoveDown": {
    message: "Mover {cidr} para baixo",
    description: "Accessible label to move an allowlist entry later",
  },
  "keyvalue.networkingInvalid": {
    message: "Informe blocos CIDR IPv4 ou IPv6 válidos e únicos.",
    description: "Validation error for Key Value allowlist entries",
  },
  "keyvalue.networkingSave": {
    message: "Salvar lista de permissões",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "Lista de permissões atualizada.",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "Não foi possível salvar a lista de permissões. Tente novamente.",
    description: "Toast when saving the allowlist fails",
  },
  "keyvalue.nameEdit": {
    message: "Editar nome do Key Value",
    description: "Accessible label for the Key Value name edit-in-place pencil",
  },
  "keyvalue.nameDescription": {
    message:
      "Altere o nome de exibição. O ID do Key Value e todos os detalhes de conexão permanecem os mesmos.",
    description: "Key Value detail rename row description",
  },
  "keyvalue.nameSuccess": {
    message: "Store Key Value renomeado para {name}.",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message: "Não foi possível renomear o store Key Value. Tente novamente.",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message: "Já existe um store Key Value com esse nome neste workspace.",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message:
      "Informe um nome de até 100 caracteres, sem quebras de linha ou caracteres de controle.",
    description: "Key Value rename validation message",
  },
  "keyvalue.detailNavLabel": {
    message: "Navegação de detalhes do Key Value",
    description: "aria-label for the detail-page tab nav",
  },
  "keyvalue.sectionNavigation": {
    message: "Seções do Key Value",
    description:
      "Accessible label for the key-value detail page section navigation",
  },
  "keyvalue.dangerZoneTitle": {
    message: "Zona de perigo",
    description:
      "Key-value detail section-nav label for the destructive actions",
  },
  "keyvalue.overviewTab": {
    message: "Visão geral",
    description: "Detail-page Overview tab label",
  },
  "keyvalue.logsTab": {
    message: "Logs",
    description: "Detail-page Logs tab label",
  },
  "keyvalue.metricsTab": {
    message: "Métricas",
    description: "Detail-page Metrics tab label",
  },
  "keyvalue.logsRangeLabel": {
    message: "Intervalo de tempo",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsAllInstances": {
    message: "Todas as instâncias",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "Pesquisar logs…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "Carregando logs…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "Nenhuma linha de log",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "Nenhuma linha de log neste intervalo de tempo. O Valkey registra eventos do espaço de chaves, consultas lentas e mensagens de inicialização.",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "Logs indisponíveis",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "A fonte de logs da plataforma não está configurada. Contate o administrador da plataforma para ativar BEX_LOKI_URL.",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "Acesso negado",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "Você não tem permissão para ver os logs deste store Key Value.",
    description: "Empty state body for a 403 on the logs query",
  },
};

export default ptKeyValue;
