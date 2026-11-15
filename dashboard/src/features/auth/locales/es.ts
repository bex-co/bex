import type { TranslationEntry } from "@/i18n";

const esAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "Te damos la bienvenida de nuevo",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "Inicia sesión en tu cuenta",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "Crea tu cuenta",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "Introduce tus datos para empezar",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "Restablece tu contraseña",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message:
      "Introduce el correo de tu cuenta para recibir un código de acceso de un solo uso",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "Establece una nueva contraseña",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "Verifica tu correo",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message:
      "Introduce la dirección de correo asociada a tu cuenta para continuar",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.verificationRequiredSubtitle": {
    message: "Verifica tu dirección de correo para seguir usando bex",
    description:
      "Verification page hero subtitle for a signed-in session whose email is not verified yet (ADR075 D8 verification wall)",
  },
  "auth.verificationWrongAccount": {
    message: "¿No es tu cuenta?",
    description:
      "Verification wall: lead-in before the sign-out link, shown only to a signed-in session",
  },
  "auth.verificationSignOut": {
    message: "Cerrar sesión",
    description:
      "Verification wall: sign-out link, shown only to a signed-in session",
  },
  "auth.settingsTitle": {
    message: "Configuración",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "Administra tu identidad, integraciones, acceso y seguridad.",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "Secciones de configuración",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "Cuenta",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message:
      "Actualiza tu perfil, tu contraseña y la autenticación en dos pasos.",
    description: "Settings page account section description",
  },
  "auth.integrationsSection": {
    message: "Integraciones",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "Conecta el control de código fuente.",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "Credenciales de acceso",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "Administra el acceso programático y las identidades SSH.",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "Seguridad",
    description:
      "Settings page section heading grouping the account's security cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message:
      "Controles de seguridad de la cuenta: agentes conectados y sesiones activas.",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "Zona de peligro",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "Elimina tu cuenta de forma permanente y revoca todo el acceso.",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "Eliminar cuenta",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "Esto elimina de forma permanente tu identidad, tus credenciales y los espacios de trabajo en los que eres el único miembro. Los espacios de trabajo compartidos siguen disponibles para sus otros administradores.",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "Se eliminarán estos espacios de trabajo",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "Saldrás de estos espacios de trabajo",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "Resuelve primero la propiedad del espacio de trabajo",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "Cada espacio de trabajo de abajo tiene otros miembros pero ningún otro administrador. Promueve a otro miembro, quita a los demás miembros o elimina el espacio de trabajo antes de eliminar tu cuenta.",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "Abrir configuración",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "Escribe {phrase} abajo para confirmar.",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "Eliminar mi cuenta",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "No se pudo iniciar la eliminación de la cuenta",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message:
      "No pudimos iniciar la eliminación de la cuenta. Inténtalo de nuevo.",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "Vista previa de la eliminación no disponible",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "No pudimos determinar con seguridad qué pasará con tus espacios de trabajo. No se ha eliminado nada.",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "Intentar de nuevo",
    description: "Account deletion preview retry button",
  },
  "auth.errorTitle": {
    message: "Algo salió mal",
    description:
      "Title of /auth/error, where Kratos sends a self-service error (w5/161)",
  },
  "auth.errorSubtitle": {
    message: "No pudimos completar ese paso.",
    description: "Subtitle of /auth/error",
  },
  "auth.errorGeneric": {
    message:
      "El paso de inicio de sesión caducó o no se pudo completar. Empieza de nuevo desde la página de inicio de sesión.",
    description:
      "/auth/error body when Kratos's error has no readable reason or the id is unknown",
  },
  "auth.errorBackToSignIn": {
    message: "Volver a iniciar sesión",
    description: "/auth/error button to the sign-in page",
  },
  "auth.errorFlowExpired": {
    message:
      "Ese paso de inicio de sesión tardó demasiado y caducó. Empieza de nuevo desde la página de inicio de sesión.",
    description: "/auth/error message for Kratos self_service_flow_expired",
  },
  "auth.errorCsrf": {
    message:
      "La comprobación de seguridad de tu navegador no coincidió, normalmente por una segunda pestaña o por cookies borradas. Empieza de nuevo desde la página de inicio de sesión.",
    description: "/auth/error message for Kratos security_csrf_violation",
  },
  "auth.errorIdentityMismatch": {
    message:
      "Ese paso pertenece a una cuenta distinta de la que tiene la sesión iniciada. Cierra sesión y empieza de nuevo.",
    description: "/auth/error message for Kratos security_identity_mismatch",
  },
  "auth.errorReturnToForbidden": {
    message:
      "La dirección a la que volver tras iniciar sesión no está permitida.",
    description:
      "/auth/error message for Kratos self_service_flow_return_to_forbidden",
  },
  "auth.accountDeletedTitle": {
    message: "Eliminación de la cuenta iniciada",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "Has cerrado sesión de forma segura.",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "Se están eliminando tus espacios de trabajo y credenciales. Puedes cerrar esta página; no hace falta ninguna otra acción.",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "Volver a bex",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "Cerrando sesión...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "Finalizando tu sesión, un momento.",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "Sesión cerrada",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "Redirigiendo al inicio de sesión…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "No se pudo cerrar la sesión",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "No pudimos finalizar tu sesión. Puede que sigas con la sesión iniciada; inténtalo de nuevo.",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "Intentar de nuevo",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "¿Cerrar sesión?",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "¿Seguro que quieres finalizar tu sesión?",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "Cerrar sesión",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "Cancelar",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "Cerrar sesión",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "Seguro por defecto",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "Las sesiones las gestiona Ory Kratos, una infraestructura de identidad probada en producción, no un sistema de autenticación hecho a mano.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "Un panel para todos los servicios",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message:
      "Despliega, supervisa y administra todo lo que se ejecuta en bex desde un solo lugar.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "Construido en abierto",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex es la alternativa a Render de código abierto.",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "Autorizar acceso",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} solicita actuar en tu nombre en bex.",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "App de terceros no verificada",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "ID de cliente",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "Destino del código de autorización",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "Sin redirección (flujo de dispositivo)",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "Está solicitando",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "Confirmar quién eres",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "Mantener la conexión sin volver a preguntarte (refresh token)",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "Leer tu perfil básico",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "Leer tu dirección de correo",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message:
      "Leer recursos ordinarios del espacio de trabajo (servicios, logs, métricas)",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message:
      "Cambiar recursos del espacio de trabajo (desplegar, reiniciar, crear, eliminar, facturación)",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message:
      "Leer secretos y cadenas de conexión (variables de entorno, URL de bases de datos, archivos)",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeCloudAgents": {
    message:
      "Ejecutar agentes de programación en la nube en tu nombre (inicia tareas que consumen cómputo y crédito de modelo, y puede enviar una rama)",
    description: "Human description of the bex.cloud_agents OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message:
      "Acceso completo al plano de control (compatibilidad con clientes de plataforma heredados)",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "Dónde funciona el token",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "Tu aprobación se recuerda durante una hora; después, este agente volverá a preguntar.",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "Aprobar",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "Denegar",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "Algo salió mal al completar la acción. Inténtalo de nuevo.",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "La autorización caducó",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message:
      "Vuelve a iniciar la conexión desde la aplicación que te trajo aquí.",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "Solicitud de autorización no válida",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "A la solicitud de esta app le faltan parámetros de seguridad obligatorios. Contacta con el desarrollador de la app.",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "Esta app solicitó acceso a la API sin especificar qué necesita. Debe solicitar al menos uno de estos: {scopes}.",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "Autorización no disponible",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message:
      "Esto debería resolverse en breve; inténtalo de nuevo en un momento.",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "Sesión iniciada con la cuenta equivocada",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "La sesión de este navegador pertenece a una cuenta distinta de la que inició esta autorización. Cierra sesión e inténtalo de nuevo.",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "Dispositivo conectado",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "Conectar un dispositivo",
    description: "Device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message:
      "Un dispositivo en el que estás iniciando sesión quiere acceder a tu cuenta.",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "Autorizar este dispositivo",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message:
      "Confirma que el código de abajo coincide con el que aparece en tu dispositivo:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message: "Autoriza esto solo si iniciaste tú mismo este inicio de sesión.",
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
    message: "El código de dispositivo caducó",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "Esta página necesita un código de dispositivo, o el código ha caducado. Vuelve a iniciar sesión desde tu dispositivo.",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "Autorización de dispositivo no disponible",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message:
      "Esto debería resolverse en breve; inténtalo de nuevo en un momento.",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "Autorización rechazada",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "No se pudo verificar esta solicitud de dispositivo. Vuelve a iniciar sesión desde tu dispositivo.",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "La autorización en el navegador está completa.",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message:
      "Vuelve a la aplicación en la que estás iniciando sesión para continuar.",
    description: "Device authorization success page hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "Esperando la autorización del navegador…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "Autorización completada. Has iniciado sesión.",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "Puedes cerrar esta pestaña.",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "Abrir el panel",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
  "auth.profileEmailLabel": {
    message: "Correo electrónico",
    description:
      "Account settings profile form: the email field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileEmailPlaceholder": {
    message: "Introduce tu correo electrónico",
    description: "Account settings profile form: the email field placeholder",
  },
  "auth.profileNameLabel": {
    message: "Nombre",
    description:
      "Account settings profile form: the name field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileNamePlaceholder": {
    message: "Introduce tu nombre",
    description: "Account settings profile form: the name field placeholder",
  },
  "auth.inputPlaceholder": {
    message: "Introduce {placeholder}",
    description:
      "Auth forms: the placeholder of a field with no wording of its own, e.g. the password; {placeholder} is the field's label",
  },
  "auth.showPassword": {
    message: "Mostrar contraseña",
    description:
      "Accessible name of the password field toggle while the password is hidden",
  },
  "auth.hidePassword": {
    message: "Ocultar contraseña",
    description:
      "Accessible name of the password field toggle while the password is shown",
  },
};

export default esAuth;
