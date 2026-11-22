import type { TranslationEntry } from "@/i18n";

const ptDatabases: Record<string, TranslationEntry> = {
  "databases.resourceType": {
    message: "Banco de dados",
    description: "Managed Postgres resource type used in document titles",
  },
  "databases.statTotal": {
    message: "Total de bancos de dados",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "Disponível",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "Criando",
    description: "Databases page stat card label (provisioning databases)",
  },
  "databases.cardTitle": {
    message: "Bancos de dados",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "Nome",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "Status",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "Plano",
    description: "Databases table column header (compute plan / tier)",
  },
  "databases.colVersion": {
    message: "Versão",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "Armazenamento",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "Criado",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "Ações",
    description: "Databases table actions column header (screen-reader only)",
  },
  "databases.statusAvailable": {
    message: "Disponível",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "Criando",
    description: "Database status badge (provisioning)",
  },
  "databases.statusRestarting": {
    message: "Reiniciando",
    description:
      "Database status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "databases.statusUpgrading": {
    message: "Atualizando",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "Indisponível",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "Suspenso",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
  },
  "databases.statusUnknown": {
    message: "Desconhecido",
    description: "Database status badge for an unrecognized status",
  },
  "databases.errorTitle": {
    message: "Não foi possível carregar os bancos de dados",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message:
      "A requisição ao bex-api falhou. Verifique sua conexão e tente novamente.",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "Nenhum banco de dados ainda",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "Crie seu primeiro Postgres gerenciado e ele aparecerá aqui.",
    description: "Databases list empty state body",
  },
  "databases.actionsMenu": {
    message: "Ações para {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "Excluir",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "Excluir {name}?",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "Esta ação não pode ser desfeita. Tem certeza de que deseja excluir esta instância do Postgres?",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "Digite {phrase} abaixo para confirmar.",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
  },
  "databases.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "Excluir banco de dados",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "Excluindo {name}…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "Não foi possível excluir {name}. Tente novamente.",
    description: "Toast when a delete request fails",
  },
  "databases.createTitle": {
    message: "Novo Postgres",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "Provisione uma instância gerenciada do PostgreSQL.",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "Nome",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message:
      "Informe um nome de até 100 caracteres, sem quebras de linha ou caracteres de controle.",
    description: "Create-database name validation message",
  },
  "databases.fieldDatabaseName": {
    message: "Banco de dados",
    description: "Create-database physical PostgreSQL database label",
  },
  "databases.fieldDatabaseNamePlaceholder": {
    message: "orders_data",
    description: "Create-database physical PostgreSQL database placeholder",
  },
  "databases.fieldDatabaseUser": {
    message: "Usuário",
    description: "Create-database physical PostgreSQL owner-role label",
  },
  "databases.fieldDatabaseUserPlaceholder": {
    message: "orders_owner",
    description: "Create-database physical PostgreSQL owner-role placeholder",
  },
  "databases.fieldPhysicalNameReserved": {
    message: "“{name}” é reservado pelo PostgreSQL. Escolha outro nome.",
    description:
      "Create-database / add-user validation for a name the managed PostgreSQL service reserves (postgres, template0/1, streaming_replica, public, none, pg_*, cnpg_*)",
  },
  "databases.fieldPhysicalNameError": {
    message:
      "Use até 63 letras minúsculas, dígitos e sublinhados; comece com uma letra ou sublinhado.",
    description: "Create-database physical PostgreSQL identifier validation",
  },
  "databases.fieldPlan": {
    message: "Plano de computação",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldVersion": {
    message: "Versão do PostgreSQL",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "Padrão (mais recente)",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "Armazenamento",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldDiskFixed": {
    message:
      "Este plano inclui um disco fixo de {size} GB. Escolha um plano maior para definir o armazenamento.",
    description:
      "Create-database hint under a plan's fixed disk size, e.g. Free (w4/376)",
  },
  "databases.fieldDiskError": {
    message: "Informe um número inteiro entre {min} e {max} GB.",
    description:
      "Create-database validation message when the disk size is below the plan floor or above the volume cap",
  },
  "databases.fieldPublic": {
    message: "Acesso público",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message:
      "Aceita conexões com credenciais válidas de qualquer endereço IP até você restringi-las em Controle de acesso.",
    description:
      "Create-database public toggle helper text; the toggle starts on, as a Render database is reachable by default (w4/553)",
  },
  "databases.fieldPublicHintOff": {
    message:
      "As conexões externas estão bloqueadas. Serviços na rede privada ainda podem se conectar.",
    description:
      "Create-database public toggle helper text once switched off: the database is created private (w4/553)",
  },
  "databases.fieldHighAvailability": {
    message: "Alta disponibilidade",
    description: "Create-database high availability toggle label (w2/m171)",
  },
  "databases.fieldHighAvailabilityHint": {
    message:
      "Execute um standby em outro nó que assume se o primário falhar. O standby é cobrado como o primário.",
    description:
      "Create-database HA toggle helper text on a plan that offers HA",
  },
  "databases.fieldHighAvailabilityPlanUnsupported": {
    message:
      "{name} não oferece alta disponibilidade. Escolha um plano com pelo menos 1 CPU.",
    description:
      "Create-database HA toggle helper text when the selected plan is below 1 CPU (Render's rule)",
  },
  "databases.createCancel": {
    message: "Cancelar",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "Criar banco de dados",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "Criando {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "Não foi possível criar {name}. Tente novamente.",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "Limite de Postgres atingido",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "Fazer upgrade do plano",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  "databases.metaTitle": {
    message: "Detalhes",
    description: "Database detail metadata card title",
  },
  "databases.statusReasonClusterFailed": {
    message:
      "O cluster do banco de dados rejeitou sua configuração, então a alteração mais recente não pôde ser aplicada.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonClusterReadFailed": {
    message: "Não foi possível ler o estado do cluster do banco de dados.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonNetworkPolicyFailed": {
    message: "Não foi possível aplicar a política de rede do banco de dados.",
    description:
      "Why an unavailable database is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonDiskAutoscalingFailed": {
    message: "Não foi possível aplicar o escalonamento automático de disco.",
    description:
      "Why an unavailable database is unavailable: the operator's DiskAutoscalingFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExportFailed": {
    message: "Não foi possível iniciar uma exportação do banco de dados.",
    description:
      "Why an unavailable database is unavailable: the operator's ExportFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPoolerFailed": {
    message: "Não foi possível provisionar o pooler de conexões.",
    description:
      "Why an unavailable database is unavailable: the operator's PoolerFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExtensionsFailed": {
    message:
      "Não foi possível instalar as extensões gerenciadas do banco de dados, então ele ainda não está disponível.",
    description:
      "Why an unavailable database is unavailable: the operator's ExtensionsFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPostUpgradeBackupFailed": {
    message: "Não foi possível fazer o backup após a atualização de versão.",
    description:
      "Why an unavailable database is unavailable: the operator's PostUpgradeBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonMajorVersionUpgradeFailed": {
    message:
      "A atualização de versão principal falhou; o banco de dados permanece na versão atual.",
    description:
      "Why an unavailable database is unavailable: the operator's MajorVersionUpgradeFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonStorageShrinkRejected": {
    message:
      "O armazenamento do Postgres só cresce: o disco não pode ficar abaixo dos {size} GB alocados.",
    description:
      "Why an unavailable database is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonRecoveryUnavailable": {
    message:
      "O banco de dados não pode ser restaurado: nenhum armazenamento de backup está configurado.",
    description:
      "Why an unavailable database is unavailable: the operator's RecoveryUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonBackupStoreUnavailable": {
    message:
      "Não foi possível configurar o armazenamento de backup do banco de dados.",
    description:
      "Why an unavailable database is unavailable: the operator's BackupStoreUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupFailed": {
    message: "Não foi possível agendar o backup diário.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupCleanupFailed": {
    message: "Não foi possível remover o agendamento do backup diário.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.metaStatus": {
    message: "Status",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "Plano de computação",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "Versão",
    description: "Database detail metadata row label",
  },
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeAction": {
    message: "Atualizar",
    description: "Button opening the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgrading": {
    message: "Atualização em andamento",
    description:
      "Version-row status while the database is offline for pg_upgrade",
  },
  "databases.versionUpgradeTitle": {
    message: "Atualizar o PostgreSQL",
    description: "PostgreSQL major-version upgrade dialog title",
  },
  "databases.versionUpgradeDescription": {
    message:
      "Atualize este banco de dados do PostgreSQL {version} para uma versão principal mais recente e compatível.",
    description: "PostgreSQL major-version upgrade dialog description",
  },
  "databases.versionUpgradeTarget": {
    message: "Versão de destino",
    description: "PostgreSQL major-version upgrade target selector label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "O banco de dados ficará indisponível enquanto a atualização offline estiver em execução. Teste a compatibilidade antes e programe o período de inatividade; bancos de dados grandes podem levar até uma hora.",
    description:
      "Downtime warning in the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgradeErrorTitle": {
    message: "Atualização bloqueada",
    description:
      "Inline backend-guard error title in the version upgrade dialog",
  },
  "databases.versionUpgradeCancel": {
    message: "Cancelar",
    description: "PostgreSQL major-version upgrade dialog cancel button",
  },
  "databases.versionUpgradeConfirm": {
    message: "Atualizar para o PostgreSQL {version}",
    description: "PostgreSQL major-version upgrade dialog confirmation button",
  },
  "databases.versionUpgradeAccepted": {
    message: "Atualização para o PostgreSQL {version} iniciada.",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message:
      "Não foi possível iniciar a atualização do PostgreSQL. Tente novamente.",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
  },
  "databases.metaDatabaseName": {
    message: "Banco de dados",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "Usuário",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "Armazenamento",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.diskAutoscalingLabel": {
    message: "Escalonamento automático de disco",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "{current} GB atuais · {cap} GB no máximo",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "Com 90% de uso, o armazenamento cresce 50%, arredondado para cima em múltiplos de 5 GB. Os aumentos são permanentes e limitados a um a cada 12 horas.",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingCurrentSize": {
    message: "{current} GB atuais",
    description: "Current disk size while plan limits are unavailable",
  },
  "databases.diskAutoscalingPlanUnsupported": {
    message:
      "Este plano não oferece escalonamento automático de disco. Uma configuração existente pode ser desativada.",
    description: "Plan restriction beside the disk autoscaling switch",
  },
  "databases.diskAutoscalingEnabled": {
    message: "Escalonamento automático de disco ativado.",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "Escalonamento automático de disco desativado.",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message:
      "Não foi possível atualizar o escalonamento automático de disco. Tente novamente.",
    description: "Toast when the disk-autoscaling mutation fails",
  },
  "databases.metaHighAvailability": {
    message: "Alta disponibilidade",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "Acesso público",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "Host externo",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaRegion": {
    message: "Região",
    description: "Database detail metadata row label (platform placement)",
  },
  "databases.metaCreated": {
    message: "Criado",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.metaExpires": {
    message: "Expira",
    description:
      "Details row: when a free database expires (Render's expiresAt, w8/m126)",
  },
  "databases.expirySoon": {
    message:
      "Este banco de dados gratuito expira em {expiresAt} e depois deixa de atender.",
    description: "Warning within 7 days of a free database's expiry",
  },
  "databases.expiryExpired": {
    message:
      "Este banco de dados gratuito expirou e está suspenso. Ele e seus dados serão excluídos em {deleteAt}.",
    description:
      "Warning on an expired free database during its 14-day grace period",
  },
  "databases.expiryUpgrade": {
    message: "Fazer upgrade para um plano pago",
    description: "Link from the expiry warning to the Plan section",
  },
  "databases.expiryHintSoon": {
    message: "Expira em breve",
    description: "List-row hint on a free database within 7 days of expiry",
  },
  "databases.expiryHintExpired": {
    message: "Expirado",
    description: "List-row hint on an expired free database",
  },
  "databases.statusReasonFreeExpired": {
    message:
      "Este banco de dados gratuito expirou. Faça upgrade para um plano pago para restaurá-lo.",
    description:
      "Why a suspended database is suspended: its free plan expired (bex-api statusReasonCode POSTGRES_FREE_EXPIRED, w8/m126)",
  },
  "databases.yes": {
    message: "Sim",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "Não",
    description: "Metadata value for a false boolean field",
  },
  "databases.connTitle": {
    message: "Conexões",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "Strings de conexão e a senha do banco de dados. Reveladas somente quando você pedir — nunca exibidas automaticamente.",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "Revelar informações de conexão",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "Ocultar informações de conexão",
    description: "Button that clears the revealed connection info",
  },
  "databases.connLogicalReplicationTitle": {
    message: "Replicação lógica",
    description:
      "Heading of the logical-replication note on the Connections card",
  },
  "databases.connLogicalReplicationBody": {
    message:
      "Desativada por padrão. Em um workspace Pro ou superior, um banco de dados com pelo menos 10 GB de armazenamento pode publicar suas alterações para assinantes dentro ou fora do bex. Para ativá-la, entre em contato com o suporte informando o ID deste banco de dados, a credencial com que os assinantes se conectam, os esquemas que eles leem e se todas as tabelas devem ser publicadas. Depois, crie publicações com CREATE PUBLICATION … FOR TABLE como usuário deste banco de dados e conecte os assinantes à URL externa, nunca à do pooler. Ele também pode assinar outro banco de dados dentro ou fora do bex: solicite da mesma forma informando a credencial que cria as assinaturas e execute CREATE SUBSCRIPTION quando as tabelas existirem (no PostgreSQL 15 e anteriores, o suporte cria a assinatura para você).",
    description:
      "Logical-replication note on the Connections card: off by default, enabled on request (Render's rules)",
  },
  "databases.connPassword": {
    message: "Senha",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "Mostrar senha",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "Ocultar senha",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "String de conexão interna",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "String de conexão externa",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "Comando psql",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connCaLabel": {
    message: "Certificado CA do servidor",
    description: "Connection-info field label (private server CA bundle)",
  },
  "databases.connCaDownload": {
    message: "Baixar certificado CA",
    description: "Button downloading the server CA PEM file",
  },
  "databases.connCaHelp": {
    message:
      "As conexões externas verificam o certificado deste banco de dados com a CA privada do servidor (sslmode=verify-full continua obrigatório). Baixe {file} e aponte seu cliente para o arquivo salvo — para o comando psql ou a string de conexão externa, use este prefixo:",
    description:
      "Explanation above the PGSSLROOTCERT example; {file} is the downloaded CA filename",
  },
  "databases.connCaSnippet": {
    message: "Exemplo de prefixo de ambiente apontando para a CA baixada",
    description:
      "Accessible label for the copyable PGSSLROOTCERT example snippet",
  },
  "databases.connCertPendingTitle": {
    message: "O certificado publicamente confiável está sendo emitido",
    description:
      "Connections card note title while a public database's Let's Encrypt certificate is still pending (w4/m357)",
  },
  "databases.connCertPendingBody": {
    message:
      "Um certificado publicamente confiável está sendo emitido (geralmente leva alguns minutos). Até lá, conecte-se com a CA abaixo ou com sslmode=require. Depois de emitido, a URL externa não precisa de CA, como no Render; oculte e revele novamente para atualizar.",
    description:
      "Connections card note body while the public database certificate is pending",
  },
  "databases.connCertFailedTitle": {
    message: "O certificado publicamente confiável ainda não foi emitido",
    description:
      "Connections card note title when issuing a public database's certificate failed (w4/m357)",
  },
  "databases.connCertFailedBody": {
    message:
      "A emissão do certificado publicamente confiável deste banco de dados falhou, e o bex tenta novamente de forma automática. Enquanto isso, o caminho da CA funciona: conecte-se com a CA abaixo ou com sslmode=require.",
    description:
      "Connections card note body when the public database certificate failed to issue",
  },
  "databases.connErrorTitle": {
    message: "Não foi possível carregar as informações de conexão",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "O banco de dados pode ainda estar sendo provisionado, ou você pode não ter permissão para ver suas credenciais.",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "Copiado para a área de transferência",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "Não foi possível copiar para a área de transferência",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "Carregando…",
    description: "Generic loading placeholder in a detail panel",
  },
  "databases.dangerDelete": {
    message: "Excluir banco de dados",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "Reiniciar banco de dados",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "Suspender banco de dados",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "Retomar banco de dados",
    description: "Detail-page bottom action: wake a suspended database",
  },
  "databases.actionSuspend": {
    message: "Suspender",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "Retomar",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "Reiniciar",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "Suspender {name}?",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "A computação é interrompida e o banco de dados fica inacessível, mas os dados são preservados. Retome-o quando quiser para trazê-lo de volta.",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "Reiniciar {name}?",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "O primário será reiniciado. As conexões caem brevemente até ele voltar.",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "Suspendendo {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "Retomando {name}…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "Reiniciando {name}…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message: "Não foi possível concluir a ação em {name}. Tente novamente.",
    description: "Toast when a lifecycle verb fails",
  },
  "databases.recoveryTitle": {
    message: "Recuperação",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "Recuperação point-in-time e backups. A restauração sempre cria um novo banco de dados — este nunca é modificado.",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "Os backups não estão ativados neste plano, então a recuperação não está disponível. Faça upgrade para um plano com backups para ativar a recuperação point-in-time.",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryUnavailable": {
    message:
      "Não foi possível ler as informações de recuperação. Tente novamente.",
    description: "Recovery panel state when backup metadata could not be read",
  },
  "databases.recoveryEarliest": {
    message: "Ponto de restauração mais antigo",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "Ponto de restauração mais recente",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryWindowNotEstablished": {
    message: "Ainda não estabelecido",
    description: "Recovery panel value before the PITR window is established",
  },
  "databases.recoveryRestore": {
    message: "Restaurar em nova instância",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryRestoreUnavailable": {
    message:
      "A restauração fica disponível quando o primeiro ponto de recuperação for estabelecido.",
    description: "Reason restore is disabled before the PITR window opens",
  },
  "databases.recoveryCreateExport": {
    message: "Criar exportação",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "Backups",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "Nenhum backup ainda.",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExportScope": {
    message:
      "Uma exportação contém somente o banco de dados {database}. Outros bancos de dados criados nesta instância não são incluídos.",
    description:
      "Recovery panel note naming the one database a logical export contains (w8/258)",
  },
  "databases.recoveryExports": {
    message: "Exportações",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "Nenhuma exportação ainda.",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryDownloadExport": {
    message: "Baixar",
    description: "Download an available logical database export",
  },
  "databases.recoveryDownloadUnavailable": {
    message: "Esta exportação não está mais disponível para download.",
    description: "Toast when Download finds the export is no longer available",
  },
  "databases.recoveryDownloadError": {
    message: "Não foi possível obter o link de download. Tente novamente.",
    description: "Toast when the download link of an export cannot be fetched",
  },
  "databases.recoveryExportInProgress": {
    message: "Já há uma exportação em andamento.",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "A exportação ainda está sendo preparada.",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "Esta exportação expirou e seu artefato foi removido.",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "Esta exportação não está disponível para download.",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "Falha na exportação: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "Criada",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "Em execução",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "Disponível",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "Falhou",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "Expirando",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "Expirada",
    description: "Logical export status",
  },
  "databases.recoveryRestoreTitle": {
    message: "Restaurar em um novo banco de dados",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "Provisione um novo banco de dados restaurado a partir dos backups deste. A origem não é alterada.",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "Nome do novo banco de dados",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "Ponto no tempo (opcional)",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "Deixe em branco para restaurar o ponto mais recente disponível.",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreTimeZone": {
    message: "Os horários estão no seu fuso horário local ({zone}).",
    description: "Restore dialog target-time zone note",
  },
  "databases.recoveryRestoreTimeOutOfWindow": {
    message:
      "Escolha um horário entre o ponto de restauração mais antigo e dez minutos atrás.",
    description:
      "Restore dialog target-time out-of-window error (the window ends ten minutes before now, w7/m453)",
  },
  "databases.recoveryRetentionWindow_one": {
    message:
      "No plano {plan}, você pode restaurar qualquer momento do último {count} dia.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_many": {
    message:
      "No plano {plan}, você pode restaurar qualquer momento dos últimos {count} de dias.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_other": {
    message:
      "No plano {plan}, você pode restaurar qualquer momento dos últimos {count} dias.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionUpgrade_one": {
    message: "{plan} mantém {count} dia.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_many": {
    message: "{plan} mantém {count} de dias.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_other": {
    message: "{plan} mantém {count} dias.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRestoreConfirm": {
    message: "Restaurar",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "Exportação iniciada.",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "Não foi possível iniciar a exportação. Tente novamente.",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "Restaurando em {name}…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "Não foi possível restaurar. Tente novamente.",
    description: "Toast when a restore request fails",
  },
  "databases.accessTitle": {
    message: "Controle de acesso",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "Restrinja o endpoint externo, agrupe conexões e gerencie usuários do banco de dados.",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "Lista de IPs permitidos",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "Salvar regras CIDR ativa o acesso externo para os IPs de origem correspondentes. Salvar uma lista vazia bloqueia as conexões externas. O acesso interno não muda.",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListInternalOnly": {
    message:
      "As conexões externas estão desativadas. Salve uma ou mais regras de IP para ativar o acesso.",
    description: "External access is disabled; saving rules enables it",
  },
  "databases.accessAllowListEmpty": {
    message:
      "Nenhuma regra de IP. Salve uma lista vazia para bloquear as conexões externas.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "databases.accessAllowListAdd": {
    message: "Adicionar",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListDescription": {
    message: "Descrição (opcional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "databases.accessAllowListCIDR": {
    message: "Novo bloco CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "databases.accessAllowListCIDRRule": {
    message: "Bloco CIDR da regra {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "databases.accessAllowListDescriptionLabel": {
    message: "Descrição da nova regra",
    description: "Accessible label for the add-row description input",
  },
  "databases.accessAllowListDescriptionRule": {
    message: "Descrição da regra {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "databases.accessAllowListRemove": {
    message: "Remover {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListMoveUp": {
    message: "Mover {cidr} para cima",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "databases.accessAllowListMoveDown": {
    message: "Mover {cidr} para baixo",
    description: "Accessible label to move an allowlist entry later",
  },
  "databases.accessAllowListInvalid": {
    message: "Informe blocos CIDR IPv4 ou IPv6 válidos e únicos.",
    description: "Validation error for database allowlist entries",
  },
  "databases.accessAllowListSave": {
    message: "Salvar lista de permissões",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "Lista de permissões atualizada.",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "Não foi possível salvar a lista de permissões. Tente novamente.",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessCredentials": {
    message: "Credenciais",
    description:
      "Access panel section heading (Postgres credentials, Render's Credentials section)",
  },
  "databases.accessCredentialsHint": {
    message:
      "Adicione uma nova credencial padrão para rotacionar a senha do banco de dados. As informações de conexão passam a usá-la, e os serviços vinculados a adotam na próxima sincronização do Blueprint. As credenciais antigas continuam funcionando até você excluí-las.",
    description: "Access panel credentials helper text",
  },
  "databases.accessCredentialDefault": {
    message: "Padrão",
    description: "Badge on the database's default credential",
  },
  "databases.accessCredentialAdd": {
    message: "Nova credencial padrão",
    description:
      "Access panel button to create a credential that becomes the default",
  },
  "databases.accessCredentialName": {
    message: "Nome de usuário da nova credencial",
    description: "Accessible label for the new-credential username input",
  },
  "databases.accessCredentialDelete": {
    message: "Excluir {name}",
    description: "Accessible label to delete a database credential",
  },
  "databases.accessCredentialDeleteTitle": {
    message: "Excluir a credencial {name}?",
    description:
      "Title of the confirmation before deleting a database credential (w4/551)",
  },
  "databases.accessCredentialDeleteBody": {
    message:
      "Toda conexão que usa {name} falha imediatamente, inclusive os serviços cujas variáveis de ambiente contêm a string de conexão dela. Isso não pode ser desfeito.",
    description:
      "Body of the credential delete confirmation: retiring the login breaks its clients at once (w4/551)",
  },
  "databases.accessCredentialDeleteConfirm": {
    message: "Excluir credencial",
    description:
      "Confirm button of the credential delete confirmation (w4/551)",
  },
  "databases.accessCredentialDefaultLocked": {
    message:
      "A credencial padrão não pode ser excluída. Adicione primeiro uma nova credencial padrão.",
    description: "Why the delete button is disabled on the default credential",
  },
  "databases.accessCredentialCreated": {
    message:
      "{name} agora é a credencial padrão. A senha dela está nas informações de conexão.",
    description: "Toast after creating a credential",
  },
  "databases.accessCredentialError": {
    message: "Não foi possível criar a credencial. Tente novamente.",
    description: "Toast when creating a credential fails",
  },
  "databases.accessCredentialDeleted": {
    message: "Revogação solicitada para {name}.",
    description:
      "Toast after deleting a credential: the login is retired asynchronously on the next reconcile, so this confirms accepted intent, not a completed revocation",
  },
  "databases.accessCredentialDeleteError": {
    message: "Não foi possível excluir {name}. Tente novamente.",
    description: "Toast when deleting a credential fails",
  },
  "databases.accessPooler": {
    message: "Pool de conexões",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "Strings de conexão em pool roteadas pelo PgBouncer, para clientes com muitas conexões.",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "Revelar strings em pool",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message:
      "Ative o pool de conexões acima para obter strings de conexão em pool.",
    description:
      "Access panel shown when no pooler is enabled; points at the Connection Pool switch in the section heading (w2/m307)",
  },
  "databases.accessPoolerInternal": {
    message: "Conexão interna em pool",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerPlanUnsupported": {
    message:
      "Este plano não oferece pool de conexões. Escolha um plano compatível antes de ativar um pooler.",
    description: "Pooler empty state on an unsupported plan",
  },
  "databases.accessPoolerExternal": {
    message: "Conexão externa em pool",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerReplicaInternal": {
    message: "Conexão interna em pool, réplica de leitura {name}",
    description:
      "Access panel pooled-string label for a read replica's in-cluster pooled connection (w6/m458)",
  },
  "databases.accessPoolerReplicaExternal": {
    message: "Conexão externa em pool, réplica de leitura {name}",
    description:
      "Access panel pooled-string label for a read replica's public pooled connection (w6/m458)",
  },
  "databases.accessPoolerError": {
    message: "Não foi possível carregar as strings de conexão em pool.",
    description: "Toast when revealing pooled strings fails",
  },
  "databases.accessPoolerStarting": {
    message:
      "O pool de conexões está iniciando. As strings de conexão em pool aparecem aqui quando o PgBouncer estiver pronto.",
    description:
      "Access panel when pooling is on but the operator has not published the pooler hosts yet (w2/m307)",
  },
  "databases.poolerToggleLabel": {
    message: "Pool de conexões",
    description:
      "Accessible label of the Connection Pool switch (Render's Info > Connection Pool, w2/m307)",
  },
  "databases.poolerEnableTitle": {
    message: "Ativar o pool de conexões para {name}?",
    description:
      "Connection-pool enable confirmation title; {name} is the database",
  },
  "databases.poolerDisableTitle": {
    message: "Desativar o pool de conexões para {name}?",
    description:
      "Connection-pool disable confirmation title; {name} is the database",
  },
  "databases.poolerEnableBody": {
    message:
      "Ativar o pool de conexões reinicia o banco de dados, o que causa alguns minutos de inatividade. As strings de conexão em pool aparecem nesta seção quando o PgBouncer estiver pronto.",
    description:
      "Connection-pool enable confirmation body for a database without high availability (Render: enabling restarts the database)",
  },
  "databases.poolerEnableBodyHA": {
    message:
      "Ativar o pool de conexões faz failover para o standby em vez de reiniciar o banco de dados, então as conexões abertas caem brevemente. As strings de conexão em pool aparecem nesta seção quando o PgBouncer estiver pronto.",
    description:
      "Connection-pool enable confirmation body when high availability is on (Render: a failover instead of a restart)",
  },
  "databases.poolerDisableBody": {
    message:
      "Desativar o pool de conexões reinicia o banco de dados, o que causa alguns minutos de inatividade. As strings de conexão em pool deixam de funcionar.",
    description:
      "Connection-pool disable confirmation body for a database without high availability",
  },
  "databases.poolerDisableBodyHA": {
    message:
      "Desativar o pool de conexões faz failover para o standby em vez de reiniciar o banco de dados, então as conexões abertas caem brevemente. As strings de conexão em pool deixam de funcionar.",
    description:
      "Connection-pool disable confirmation body when high availability is on",
  },
  "databases.poolerEnableConfirm": {
    message: "Ativar pool de conexões",
    description: "Connection-pool enable confirmation button (Render's label)",
  },
  "databases.poolerDisableConfirm": {
    message: "Desativar pool de conexões",
    description: "Connection-pool disable confirmation button",
  },
  "databases.poolerEnableSuccess": {
    message: "O pool de conexões está sendo ativado para {name}.",
    description: "Toast after the connection-pool enable was accepted",
  },
  "databases.poolerDisableSuccess": {
    message: "O pool de conexões está sendo desativado para {name}.",
    description: "Toast after the connection-pool disable was accepted",
  },
  "databases.poolerEnableError": {
    message: "Não foi possível ativar o pool de conexões para {name}.",
    description: "Fallback toast when enabling the connection pool fails",
  },
  "databases.poolerDisableError": {
    message: "Não foi possível desativar o pool de conexões para {name}.",
    description: "Fallback toast when disabling the connection pool fails",
  },
  "databases.haTitle": {
    message: "Alta disponibilidade",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "Cluster replicado com failover automático. Quando ativada, um standby está sempre pronto para assumir.",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "Status",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "Ativada",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "Desativada",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "Acionar failover",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "Acionar failover em {name}?",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "Isso promove um standby a primário. As conexões caem brevemente durante a troca. O banco de dados continua disponível — nenhum dado é perdido.",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "Cancelar",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "Acionar failover",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "Failover acionado em {name}.",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message: "Não foi possível acionar o failover em {name}. Tente novamente.",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "Réplicas de leitura",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "Nenhuma réplica de leitura nomeada.",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "Interna",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "Externa",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haAddReplica": {
    message: "Adicionar réplica de leitura",
    description:
      "HA panel button that opens the add-read-replica dialog (w5/m138)",
  },
  "databases.haAddReplicaTitle": {
    message: "Adicionar uma réplica de leitura",
    description: "Add-read-replica dialog title",
  },
  "databases.haAddReplicaBody": {
    message:
      "Uma réplica de leitura é uma instância separada, cobrada à parte, que recebe dados do primário por streaming e atende consultas somente leitura em seu próprio endereço.",
    description: "Add-read-replica dialog description",
  },
  "databases.haReplicaNameLabel": {
    message: "Nome",
    description: "Add-read-replica dialog name field label",
  },
  "databases.haReplicaNameHint": {
    message:
      "Letras minúsculas, dígitos e hifens, até {max} caracteres. Faz parte do hostname da réplica.",
    description: "Add-read-replica name hint; max is the API's length limit",
  },
  "databases.haReplicaNameInvalid": {
    message:
      "Use de 1 a {max} letras minúsculas, dígitos ou hifens, começando e terminando com uma letra ou dígito.",
    description:
      "Add-read-replica name error; mirrors the API's POSTGRES_READ_REPLICA_NAME_INVALID",
  },
  "databases.haReplicaNameDuplicate": {
    message:
      "Este banco de dados já tem uma réplica de leitura chamada {name}.",
    description: "Add-read-replica name error for a name already in use",
  },
  "databases.haAddReplicaSubmit": {
    message: "Adicionar réplica",
    description: "Add-read-replica dialog submit button",
  },
  "databases.haAddReplicaSuccess": {
    message: "A réplica de leitura {name} está sendo criada.",
    description: "Toast after a read replica is added",
  },
  "databases.haAddReplicaError": {
    message: "Não foi possível adicionar a réplica de leitura {name}.",
    description: "Toast when adding a read replica fails",
  },
  "databases.haRemoveReplica": {
    message: "Remover",
    description: "HA panel per-replica remove button",
  },
  "databases.haRemoveReplicaTitle": {
    message: "Remover a réplica de leitura {name}?",
    description: "Remove-read-replica confirmation title",
  },
  "databases.haRemoveReplicaBody": {
    message:
      "A instância dela é excluída e suas strings de conexão deixam de funcionar. O primário não é afetado.",
    description: "Remove-read-replica confirmation body",
  },
  "databases.haRemoveReplicaConfirm": {
    message: "Remover réplica",
    description: "Remove-read-replica confirm button",
  },
  "databases.haRemoveReplicaSuccess": {
    message: "A réplica de leitura {name} está sendo removida.",
    description: "Toast after a read replica is removed",
  },
  "databases.haRemoveReplicaError": {
    message: "Não foi possível remover a réplica de leitura {name}.",
    description: "Toast when removing a read replica fails",
  },
  "databases.haReplicaPlanUnsupported": {
    message:
      "Réplicas de leitura não estão disponíveis no plano deste banco de dados.",
    description: "Why Add read replica is disabled: the plan allows none",
  },
  "databases.haReplicaLimitReached": {
    message:
      "Este banco de dados atingiu o máximo de {max} réplicas de leitura.",
    description:
      "Why Add read replica is disabled: the plan's replica limit is reached",
  },
  "databases.haReplicaStorageTooSmall": {
    message:
      "Réplicas de leitura precisam de pelo menos {min} GB de armazenamento; este banco de dados tem {current} GB.",
    description:
      "Why Add read replica is disabled: storage is below the replica minimum",
  },
  "databases.haNotEnabled": {
    message:
      "A alta disponibilidade não está ativada para este banco de dados.",
    description: "HA panel line when HA is off on a plan that offers it",
  },
  "databases.haPlanUnsupported": {
    message:
      "A alta disponibilidade exige um plano com pelo menos 1 CPU. O plano deste banco de dados não a oferece.",
    description:
      "HA panel state when HA is off and the current plan is below 1 CPU (w8/m43, Render's rule)",
  },
  "databases.haToggleLabel": {
    message: "Alta disponibilidade",
    description: "Accessible name of the HA panel's on/off switch (w2/m171)",
  },
  "databases.haStarting": {
    message:
      "Um standby está iniciando em outro nó. O failover fica disponível quando ele estiver pronto.",
    description:
      "HA panel line while HA is on but the standby is not ready yet",
  },
  "databases.haStopping": {
    message:
      "A alta disponibilidade está sendo desativada. O standby está sendo removido.",
    description: "HA panel line while HA is off but the standby still runs",
  },
  "databases.haEnableTitle": {
    message: "Ativar a alta disponibilidade em {name}?",
    description: "HA enable confirmation title",
  },
  "databases.haEnableBody": {
    message:
      "Uma instância standby inicia em outro nó e assume automaticamente se o primário falhar. Ela é cobrada como o primário.",
    description: "HA enable confirmation body",
  },
  "databases.haEnablePrice": {
    message: "O standby adiciona {price}.",
    description:
      "HA enable confirmation price note; {price} is a formatted monthly price",
  },
  "databases.haEnableConfirm": {
    message: "Ativar alta disponibilidade",
    description: "HA enable confirmation button",
  },
  "databases.haDisableTitle": {
    message: "Desativar a alta disponibilidade em {name}?",
    description: "HA disable confirmation title",
  },
  "databases.haDisableBody": {
    message:
      "O standby é removido. Se o primário falhar, o banco de dados fica indisponível até se recuperar.",
    description: "HA disable confirmation body",
  },
  "databases.haDisableConfirm": {
    message: "Desativar alta disponibilidade",
    description: "HA disable confirmation button",
  },
  "databases.haEnableSuccess": {
    message: "A alta disponibilidade está sendo ativada em {name}.",
    description: "Toast after HA was enabled",
  },
  "databases.haDisableSuccess": {
    message: "A alta disponibilidade está sendo desativada em {name}.",
    description: "Toast after HA was disabled",
  },
  "databases.haEnableError": {
    message: "Não foi possível ativar a alta disponibilidade em {name}.",
    description: "Fallback toast when enabling HA fails",
  },
  "databases.haDisableError": {
    message: "Não foi possível desativar a alta disponibilidade em {name}.",
    description: "Fallback toast when disabling HA fails",
  },
  "databases.planPickerHAUnsupported": {
    message:
      "{name} não oferece alta disponibilidade (exige pelo menos 1 CPU). Desative primeiro a alta disponibilidade.",
    description:
      "Plan picker refusal when an HA database selects a plan below 1 CPU; mirrors the API message (w8/m43)",
  },
  "databases.planPickerStorageUnsupported": {
    message:
      "{name} permite até {max} GB de armazenamento. Este banco de dados tem {current} GB, que não podem ser reduzidos. Escolha um plano com armazenamento suficiente.",
    description: "Plan change blocked by the database's existing storage",
  },
  "databases.planPickerReplicasUnsupported": {
    message:
      "Limite de réplicas de leitura para {name}: {max}. Este banco de dados tem {current}.",
    description: "Plan change blocked by the database's read replicas",
  },
  "databases.planPickerAutoscalingUnsupported": {
    message:
      "{name} não oferece escalonamento automático de disco. Desative primeiro o escalonamento automático de disco.",
    description: "Plan change blocked while disk autoscaling is enabled",
  },
  "databases.planPickerPoolerUnsupported": {
    message: "{name} não oferece pool de conexões. Desative primeiro o pooler.",
    description: "Plan change blocked while a connection pooler is enabled",
  },
  "databases.planTitle": {
    message: "Plano de computação",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "Altere o plano de computação. Um banco de dados sem standby reinicia para aplicar a mudança e fica indisponível por um ou dois minutos. Seus dados são mantidos.",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "Salvar",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "Cancelar",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.planPickerConfirmTitle": {
    message: "Mudar para {name}?",
    description: "Database plan-picker confirm dialog title",
  },
  "databases.planPickerConfirmBody": {
    message:
      "O banco de dados reinicia para aplicar o novo plano de computação e fica indisponível por um ou dois minutos. As conexões caem. Seus dados são mantidos.",
    description: "Database plan-picker confirm dialog body",
  },
  "databases.planPickerConfirmPrice": {
    message: "O novo plano custa ${price}/mês.",
    description:
      "Plan-change confirm: the new plan's monthly price, from the API's price sheet (w4/156)",
  },
  "databases.planPickerSuccess": {
    message: "Atualizando o plano para {name}…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "Não foi possível atualizar o plano. Tente novamente.",
    description: "Toast when a plan update fails",
  },
  "databases.storageTitle": {
    message: "Armazenamento",
    description:
      "Plan section Storage row heading (Render's Plan page Storage section, w2/m307)",
  },
  "databases.storageSize": {
    message: "{size} GB",
    description: "Plan section Storage row: the allocated storage",
  },
  "databases.storageFixed": {
    message:
      "Bancos de dados gratuitos têm armazenamento fixo. Faça upgrade para um plano pago para adicionar armazenamento.",
    description:
      "Why the Storage Edit button is disabled on a free database (bex-api POSTGRES_STORAGE_PLAN_UNSUPPORTED)",
  },
  "databases.storageEdit": {
    message: "Editar",
    description:
      "Plan section Storage row button that opens the storage dialog",
  },
  "databases.storageEditAria": {
    message: "Editar armazenamento",
    description: "Accessible name of the Storage row's Edit button",
  },
  "databases.storageEditTitle": {
    message: "Editar armazenamento de {name}",
    description: "Storage dialog title; {name} is the database",
  },
  "databases.storageEditBody": {
    message:
      "Informe qualquer múltiplo de {step} GB maior que os {current} GB atuais. O armazenamento não pode ser reduzido depois de aumentar.",
    description:
      "Storage dialog description (Render: storage only grows, in 5 GB steps)",
  },
  "databases.storageEditLabel": {
    message: "Armazenamento (GB)",
    description: "Storage dialog size field label",
  },
  "databases.storageEditStep": {
    message: "Informe um múltiplo inteiro de {step} GB.",
    description: "Storage dialog validation: not a whole multiple of 5 GB",
  },
  "databases.storageEditNotGreater": {
    message:
      "Informe mais que os {current} GB atuais. O armazenamento não pode ser reduzido.",
    description:
      "Storage dialog validation: the size is not above the current storage",
  },
  "databases.storageEditTooLarge": {
    message: "{name} permite até {max} GB de armazenamento.",
    description:
      "Storage dialog validation: the size is above the plan's maximum; {name} is the plan",
  },
  "databases.storageEditSave": {
    message: "Salvar",
    description: "Storage dialog submit button",
  },
  "databases.storageEditSuccess": {
    message: "Aumentando o armazenamento para {size} GB…",
    description: "Toast after a storage increase is accepted",
  },
  "databases.storageEditError": {
    message: "Não foi possível atualizar o armazenamento. Tente novamente.",
    description:
      "Storage dialog fallback error when the server gives no reason",
  },
  "databases.nameEdit": {
    message: "Editar nome do banco de dados",
    description: "Accessible label for the database-name edit-in-place pencil",
  },
  "databases.nameDescription": {
    message:
      "Altere o nome de exibição. O ID do banco de dados e todos os detalhes de conexão permanecem os mesmos.",
    description: "Database detail rename row description",
  },
  "databases.nameSuccess": {
    message: "Banco de dados renomeado para {name}.",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "Não foi possível renomear o banco de dados. Tente novamente.",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message: "Já existe um banco de dados com esse nome neste workspace.",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message:
      "Informe um nome de até 100 caracteres, sem quebras de linha ou caracteres de controle.",
    description: "Database rename validation message",
  },
  "databases.insightsTitle": {
    message: "Insights",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message:
      "Introspecção ao vivo de processos, desempenho de consultas, armazenamento e configuração.",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "Atualizar insights",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message:
      "Indisponível — o banco de dados pode ainda estar sendo provisionado.",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "Armazenamento",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "Ainda não há dados de tamanho.",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "Processos ativos",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "Nenhum processo ativo.",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "Principais consultas",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "Nenhuma consulta ainda. As instruções executadas pelo seu aplicativo e pelo console SQL aparecerão aqui.",
    description:
      "Insights top-queries empty state: the read succeeded and nothing the owner ran is in the list. A failed read, including a missing pg_stat_statements, shows insightsUnavailable instead, so this text must not blame the extension",
  },
  "databases.insightsQueryHidden": {
    message: "Oculto",
    description:
      "Insights — query cell for a row whose SQL PostgreSQL hides from the reading role",
  },
  "databases.insightsMaskedNote": {
    message:
      "Algumas linhas pertencem a outras roles do banco de dados, então o PostgreSQL oculta o SQL delas nesta visão. Todo o resto da linha é real.",
    description:
      "Insights — explanation shown when at least one process or top-query row has masked SQL",
  },
  "databases.insightsTableScansTitle": {
    message: "Varreduras de tabela",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "Ainda não há tabelas de usuário com estatísticas de varredura.",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsDeclaredValue": {
    message: "Valor declarado",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedValue": {
    message: "Configuração observada",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedHint": {
    message:
      "Configuração ao vivo; isso não confirma que o valor solicitado foi aplicado.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsNotObserved": {
    message:
      "A substituição solicitada não foi observada. Ela pode estar pendente ou ser inválida.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservationUnavailable": {
    message: "Não é possível verificar esta substituição agora.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsTitle": {
    message: "Substituições de parâmetros",
    description:
      "Insights panel — heading for the parameters this database declares (read-only)",
  },
  "databases.insightsParamsDescription": {
    message:
      "Configurações que este banco de dados declarou antes de as substituições de parâmetros serem descontinuadas. Elas ainda se aplicam, mas não podem mais ser alteradas aqui; defina padrões de sessão com ALTER DATABASE … SET no console SQL.",
    description:
      "Explains that the declared parameters are read-only legacy settings that still apply, and how to set session defaults instead",
  },
  "databases.insightsEffectiveParamsTitle": {
    message: "Parâmetros fora do padrão",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsEffectiveParamsDescription": {
    message:
      "Todas as configurações atualmente em vigor que não estão no padrão, inclusive as gerenciadas pela plataforma. Somente leitura.",
    description:
      "Explains that the pg_settings table is observed configuration, mostly platform-owned, and cannot be edited",
  },
  "databases.insightsNoParams": {
    message: "Todos os parâmetros estão nos valores padrão.",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "Tabela",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColRelation": {
    message: "Tabela ou índice",
    description:
      "Insights sizes column header (a table's or an index's name; tables and indexes are listed together)",
  },
  "databases.insightsIndexOn": {
    message: "índice em {table}",
    description:
      "Insights sizes: suffix naming the table an index row belongs to; {table} is the quoted schema-qualified table name",
  },
  "databases.insightsColSize": {
    message: "Tamanho",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "Usuário",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "Estado",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "Duração",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "Consulta",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "Chamadas",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "Tempo total",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "Tempo médio",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "Varreduras seq.",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "Varreduras de índice",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "Linhas ativas",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "Parâmetro",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "Configuração",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "Origem",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "Console SQL",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "Execute uma única instrução SQL neste banco de dados. Os resultados são limitados a 500 linhas e as consultas expiram após 10 segundos.",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "Consulta SQL",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Pressione Ctrl+Enter ou ⌘+Enter para executar",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlCreating": {
    message:
      "Este banco de dados ainda está sendo criado. O SQL é executado quando ele estiver Disponível; sua consulta e o histórico são mantidos.",
    description: "SQL console notice while the database is creating (w4/177)",
  },
  "databases.sqlUpgrading": {
    message:
      "Uma atualização offline do PostgreSQL está em andamento. O SQL volta a ser executado quando o banco de dados estiver Disponível; sua consulta e o histórico são mantidos.",
    description: "SQL console notice while the database is upgrading (w4/177)",
  },
  "databases.sqlRestarting": {
    message:
      "Este banco de dados está reiniciando. Aguarde até ele estar Disponível para executar SQL; sua consulta e o histórico são mantidos.",
    description: "SQL console notice while the database is restarting (w4/177)",
  },
  "databases.sqlSuspended": {
    message:
      "Este banco de dados está suspenso. Retome-o para executar SQL; sua consulta e o histórico são mantidos.",
    description: "SQL console notice while the database is suspended",
  },
  "databases.sqlRun": {
    message: "Executar consulta",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "Executando…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "Não foi possível executar a consulta. Tente novamente.",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows_one": {
    message: "{count} linha retornada",
    description: "SQL console SELECT result count (singular)",
  },
  "databases.sqlReturnedRows_many": {
    message: "{count} de linhas retornadas",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlReturnedRows_other": {
    message: "{count} linhas retornadas",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlAffectedRows_one": {
    message: "{count} linha afetada",
    description: "SQL console write result count (singular)",
  },
  "databases.sqlAffectedRows_many": {
    message: "{count} de linhas afetadas",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlAffectedRows_other": {
    message: "{count} linhas afetadas",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlTruncated": {
    message: "Resultado limitado a {limit} linhas",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNullLabel": {
    message: "SQL NULL (sem valor)",
    description:
      "SQL console accessible meaning of a null cell, distinct from the literal text NULL",
  },
  "databases.sqlNoRows": {
    message: "A consulta não retornou nenhuma linha.",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "Anterior",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "Próxima",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "Página {page} de {pages}",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "Histórico de consultas",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "Limpar",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "As consultas executadas nesta sessão do navegador aparecem aqui.",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "Executar uma instrução que pode modificar dados?",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "Esta instrução não é reconhecida como somente leitura. Ela pode alterar ou excluir dados do banco de dados e não pode ser desfeita pelo bex.",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "Cancelar",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "Executar instrução",
    description: "SQL console confirmed write action",
  },
  "databases.detailNavLabel": {
    message: "Detalhes do banco de dados",
    description: "Accessible label for the managed Postgres detail tabs",
  },
  "databases.sectionNavigation": {
    message: "Seções do banco de dados",
    description:
      "Accessible label for the database detail page section navigation",
  },
  "databases.dangerZoneTitle": {
    message: "Zona de perigo",
    description:
      "Database detail section-nav label for the destructive actions",
  },
  "databases.overviewTab": {
    message: "Visão geral",
    description: "Managed Postgres overview tab",
  },
  "databases.logsTab": {
    message: "Logs",
    description: "Managed Postgres logs tab",
  },
  "databases.logsRangeLabel": {
    message: "Intervalo de tempo",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsInstanceLabel": {
    message: "Instância do banco de dados",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "Todas as instâncias",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "Pesquisar logs do banco de dados",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "Carregando logs do banco de dados…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "Nenhum log do banco de dados ainda",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "A saída do Postgres neste intervalo de tempo aparecerá aqui.",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "Os logs do banco de dados não estão configurados",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "Esta instalação não tem histórico de logs durável nem uma fonte de logs de pods do CNPG ao vivo configurada.",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "Você não pode ver estes logs",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message:
      "Os logs do banco de dados exigem acesso de colaborador neste workspace.",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "Não foi possível carregar os logs do banco de dados",
    description: "Managed Postgres logs generic error title",
  },
  "databases.fieldNameDescription": {
    message: "Um nome único para sua instância Postgres.",
    description: "Create-Postgres Name row description",
  },
  "databases.fieldDatabaseNameDescription": {
    message:
      "O dbname do Postgres, definido apenas na criação. Deixe em branco para gerar um.",
    description:
      "Create-Postgres Database row description: the physical dbname, create-time only",
  },
  "databases.fieldDatabaseUserDescription": {
    message:
      "O usuário do Postgres dono do banco de dados, definido apenas na criação. Deixe em branco para gerar um.",
    description:
      "Create-Postgres User row description: the physical owner role, create-time only",
  },
  "databases.fieldPlanDescription": {
    message:
      "Defina a RAM e a CPU do seu banco de dados. Você pode alterar a computação depois.",
    description: "Create-Postgres plan row description (Render's Compute)",
  },
  "databases.fieldDiskDescription": {
    message:
      "A capacidade do seu banco de dados, em GB. Você pode aumentá-la depois, mas não reduzi-la.",
    description:
      "Create-Postgres Storage row description (Render: storage only grows)",
  },
  "databases.fieldDiskUnit": {
    message: "GB",
    description: "Unit after the create-Postgres storage input",
  },
  "databases.fieldVersionDescription": {
    message: "A versão principal do PostgreSQL que o banco de dados executa.",
    description: "Create-Postgres version row description",
  },
  // --- Apps: admin apps (Render's Postgres Apps page, w4/m551) ---
  "databases.adminAppsTitle": {
    message: "Apps",
    description:
      "Database page Apps section title and navigation label (Render's Postgres Apps page)",
  },
  "databases.adminAppsDescription": {
    message:
      "Implante um app de administração como serviço web na região e no ambiente deste banco de dados, conectado a ele pela rede privada.",
    description: "Database page Apps section description",
  },
  "databases.adminAppsErrorTitle": {
    message: "Não foi possível carregar os apps",
    description: "Database Apps section load error title",
  },
  "databases.adminAppsErrorBody": {
    message:
      "Algo deu errado ao carregar os apps deste banco de dados. Tente novamente em instantes.",
    description: "Database Apps section load error body",
  },
  "databases.adminAppPgAdminDescription": {
    message:
      "Administração geral do PostgreSQL. Gerencie schemas, tabelas e índices. Execute consultas avulsas e veja seus planos de consulta.",
    description: "pgAdmin card description (Render's copy)",
  },
  "databases.adminAppPgHeroDescription": {
    message:
      "Um painel de desempenho para o seu banco de dados. Monitore o uso de recursos, analise conexões ativas e inspecione consultas recentes para identificar gargalos.",
    description: "PgHero card description (Render's copy)",
  },
  "databases.adminAppDocs": {
    message: "Ler a documentação",
    description: "Admin app card link to the app's documentation",
  },
  "databases.adminAppDeploy": {
    message: "Implantar app",
    description: "Admin app card button that opens the deploy dialog",
  },
  "databases.adminAppOpen": {
    message: "Abrir app",
    description:
      "Deployed admin app card button that opens the app in a new tab",
  },
  "databases.adminAppManage": {
    message: "Gerenciar",
    description: "Deployed admin app card link to its service page",
  },
  "databases.adminAppDialogTitle": {
    message: "Criar um app {app}",
    description: "Admin app deploy dialog title; {app} is pgAdmin or PgHero",
  },
  "databases.adminAppDialogDescription": {
    message:
      "Isso implantará um novo serviço web na mesma região e ambiente do seu banco de dados. Você pode configurá-lo como qualquer outro serviço web.",
    description: "Admin app deploy dialog body",
  },
  "databases.adminAppAuthentication": {
    message: "Autenticação",
    description: "Admin app deploy dialog login section heading",
  },
  "databases.adminAppAuthenticationHint": {
    message:
      "Geramos automaticamente um nome de usuário e uma senha para a sua instância do {app}. Você pode personalizá-los abaixo.",
    description:
      "Admin app deploy dialog login section text; {app} is the app name",
  },
  "databases.adminAppUsername": {
    message: "Nome de usuário",
    description: "Admin app deploy dialog username field label",
  },
  "databases.adminAppPassword": {
    message: "Senha",
    description: "Admin app deploy dialog password field label",
  },
  "databases.adminAppShow": {
    message: "Mostrar {field}",
    description:
      "Accessible label of the button that unmasks a login field; {field} is its label",
  },
  "databases.adminAppHide": {
    message: "Ocultar {field}",
    description:
      "Accessible label of the button that masks a login field; {field} is its label",
  },
  "databases.adminAppMonthlyCharges": {
    message: "Cobranças mensais",
    description: "Admin app deploy dialog billing section heading",
  },
  "databases.adminAppMonthlyChargesHint": {
    message:
      "Instâncias de serviço web são cobradas proporcionalmente por segundo. As cobranças abaixo refletem apenas as do {app}.",
    description:
      "Admin app deploy dialog billing section text; {app} is the app name",
  },
  "databases.adminAppBillingStarter": {
    message: "Serviço web - instância Starter",
    description:
      "Admin app deploy dialog billing line for a Starter web service",
  },
  "databases.adminAppBillingFree": {
    message: "Serviço web - instância Free",
    description: "Admin app deploy dialog billing line for a Free web service",
  },
  "databases.adminAppBillingDisk": {
    message: "Disco - {size} GB",
    description:
      "Admin app deploy dialog billing line for its disk; {size} is in GB",
  },
  "databases.adminAppBillingPrice": {
    message: "${usd} / mês",
    description:
      "Admin app deploy dialog billing line price; {usd} is a dollar amount",
  },
  "databases.adminAppDeploySubmit": {
    message: "Implantar {app}",
    description: "Admin app deploy dialog submit button; {app} is the app name",
  },
  "databases.adminAppDeployed": {
    message: "{app} implantado em {service}.",
    description:
      "Toast after an admin app deploys; {service} is its new service's name",
  },
  "databases.adminAppDeployError": {
    message: "Não foi possível criar o app. Tentar novamente?",
    description: "Admin app deploy dialog generic failure",
  },
  "databases.adminAppPaymentRequired": {
    message:
      "Adicione um método de pagamento a este workspace para implantar o {app}.",
    description:
      "Admin app deploy dialog message when billing onboarding was closed without a payment method",
  },
  "databases.adminAppErrorTitle": {
    message: "Erro",
    description: "Admin app deploy dialog error title",
  },
};

export default ptDatabases;
