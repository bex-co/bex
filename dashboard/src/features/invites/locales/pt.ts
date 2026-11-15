import type { TranslationEntry } from "@/i18n";

const ptInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "Convite para o workspace",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message:
      "Crie uma conta ou entre para analisar seu convite para o workspace.",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "Cadastrar-se",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "Entrar",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "Participar de {workspace}",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "Você é membro de {workspace}",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "Você entrará como {role}.",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "Sua função é {role}.",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "Convidado por {email}",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "Conectado como {email}",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "Participar do workspace",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "Entrando…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "Abrir workspace",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "Abrindo…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "Agora não",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "Continuar para o bex",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "Convite indisponível",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "Este link de convite é inválido ou foi revogado. Peça um novo convite ao administrador do workspace.",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "Este convite expirou. Peça ao administrador do workspace para reenviá-lo.",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "Este convite já foi usado. Entre com a conta que participou ou peça um novo convite.",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "Este workspace não pode aceitar mais um membro no plano atual. Peça ao administrador do workspace para verificar o plano e os assentos e tente novamente.",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "Não foi possível concluir essa solicitação. Seu convite está salvo nesta aba. Tente novamente.",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "Sua participação foi confirmada, mas o acesso ao workspace ainda não está pronto. Tente abrir o workspace novamente.",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "Não foi possível salvar seu convite neste navegador. Permita o armazenamento do site e reabra o link do convite.",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "Tentar novamente",
    description: "Invitation review: retry",
  },
};

export default ptInvites;
