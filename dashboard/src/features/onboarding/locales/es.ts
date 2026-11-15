import type { TranslationEntry } from "@/i18n";

const es: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "Añade un método de pago",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message:
      "Un último paso antes de que tu espacio de trabajo pueda ejecutar algo",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "Se requiere un método de pago",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "bex alojado es un producto de pago: debe haber un método de pago registrado antes de que este espacio de trabajo pueda crear o ejecutar cualquier recurso, incluidos los del nivel gratuito. Solo pagas por lo que usas.",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "Espacio de trabajo: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "Método de pago recibido: confirmando con Stripe…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message: "Se canceló el pago. No se añadió ningún método de pago.",
    description: "Notice after returning from a canceled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "¿Prefieres no añadir una tarjeta? bex es de código abierto: ejecútalo en tu propia infraestructura, gratis y sin límites.",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "Eliminar esta cuenta y autoalojar",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "¿Eliminar esta cuenta y autoalojar?",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "Esto elimina de forma permanente tu cuenta de bex y tu espacio de trabajo. Nunca se te cobró y no se conserva nada. Te enviaremos a la guía de autoalojamiento: bex se ejecuta gratis en tu propia infraestructura, sin límites.",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "Eliminar cuenta y continuar",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "Conservar mi cuenta",
    description:
      "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "No se pudo eliminar la cuenta. No se cambió nada.",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "Cerrar sesión",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "Intentar de nuevo",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "Continuar al panel",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "Continuando…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
  "onboarding.gettingStartedTitle": {
    message: "Empieza con bex",
    description:
      "Getting-started checklist title on an empty workspace Overview",
  },
  "onboarding.gettingStartedSubtitle": {
    message: "Unos pocos pasos hasta tu primer despliegue.",
    description: "Getting-started checklist subtitle",
  },
  "onboarding.gettingStartedDismiss": {
    message: "Descartar la guía de inicio",
    description:
      "Accessible label of the getting-started checklist's dismiss button",
  },
  "onboarding.gettingStartedDone": {
    message: "Hecho",
    description: "Accessible label of a completed checklist item's icon",
  },
  "onboarding.gettingStartedPending": {
    message: "Aún sin hacer",
    description: "Accessible label of a pending checklist item's icon",
  },
  "onboarding.gettingStarted.createService": {
    message: "Crea tu primer servicio",
    description: "Checklist item linking to the new-service flow",
  },
  "onboarding.gettingStarted.connectGithub": {
    message: "Conecta GitHub para desplegar desde un repositorio",
    description: "Checklist item linking to Settings → Integrations",
  },
  "onboarding.gettingStarted.inviteTeammate": {
    message: "Invita a un compañero de equipo",
    description: "Checklist item linking to workspace settings (team)",
  },
  "onboarding.gettingStarted.upgradeToInvite": {
    message: "Mejora tu plan para invitar a compañeros de equipo",
    description:
      "Checklist item shown instead of Invite a teammate while every seat is used (Hobby); opens the plan change",
  },
};

export default es;
