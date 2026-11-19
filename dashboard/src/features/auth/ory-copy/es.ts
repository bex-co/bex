import type { OryCopy } from "./keys";

// Ory Elements' own `es` catalog entries, copied verbatim (see ./keys.ts).
const es: OryCopy = {
  "orySettings.profileTitle": "Configuración de Perfil",
  "orySettings.profileDescription": "Actualiza la información de tu perfil",
  "orySettings.passwordTitle": "Cambiar Contraseña",
  "orySettings.passwordDescription": "Modifica tu contraseña",
  "orySettings.passwordLabel": "Contraseña",
  "orySettings.oidcTitle": "Inicio de Sesión Social",
  "orySettings.oidcDescription":
    "Administra la configuración de tu inicio de sesión social",
  "orySettings.oidcInfo":
    "Las cuentas conectadas de estos proveedores se pueden utilizar para iniciar sesión en tu cuenta",
  "orySettings.lookupSecretTitle": "Códigos de Recuperación",
  "orySettings.lookupSecretDescription":
    "Gestiona tus códigos de recuperación de respaldo 2FA",
  "orySettings.lookupSecretEnable":
    "Generar nuevos códigos de recuperación de respaldo",
  "orySettings.webauthnTitle": "Gestionar Tokens de Hardware",
  "orySettings.webauthnDescription":
    "Administra la configuración de tu token de hardware",
  "orySettings.webauthnNameLabel": "Nombre de la llave de seguridad",
  "orySettings.webauthnAdd": "Agregar llave de seguridad",
  "orySettings.webauthnInfo":
    "Los tokens de hardware se utilizan para la autenticación de segundo factor o como primer factor con las claves de acceso",
  "orySettings.totpTitle": "Aplicación Autenticadora",
  "orySettings.totpDescription":
    "Agrega una aplicación de autenticación TOTP a tu cuenta para mejorar la seguridad de tu cuenta. Las aplicaciones de autenticación populares son LastPass y Google Authenticator.",
  "orySettings.totpSecretLabel":
    "Este es el secreto de su app autenticadora. Úselo si no puede escanear el código QR.",
  "orySettings.totpCodeLabel": "Verificar código",
  "orySettings.totpInfoNotLinked":
    "Para habilitar, escanea el código QR con tu autenticador e ingresa el código.",
  "orySettings.save": "Guardar",
  "oryCard.loginTitle": "Iniciar sesión",
  "oryCard.loginSubtitle": "Iniciar sesión con {parts}",
  "oryCard.registrationTitle": "Registrar una cuenta",
  "oryCard.registrationSubtitle": "Registrarse con {parts}",
  "oryCard.passwordPart": "tu {identifierLabel} y contraseña",
  "oryCard.oidcPart": "un proveedor social",
  "oryCard.identifierPart": "tu {identifierLabel}",
  "oryCard.or": "o",
  "oryCard.recoveryTitle": "Recuperar su cuenta",
  "oryCard.recoverySubtitle":
    "Ingresa el identificador asociado con tu cuenta para recibir un código de acceso único",
  "oryCard.verificationTitle": "Verificar su cuenta",
  "oryCard.verificationSubtitle":
    "Ingresa la dirección de correo electrónico asociada con tu cuenta para verificarla",
  "oryCard.loginFooterLabel": "¿No tiene una cuenta?",
  "oryCard.loginFooterLink": "Registrarse",
  "oryCard.registrationFooterLabel": "¿Ya tiene una cuenta?",
  "oryCard.registrationFooterLink": "Iniciar sesión",
  "oryCard.recoverAccount": "Recuperar cuenta",
  "oryCard.loginSubmit": "Iniciar sesión con contraseña",
  "oryCard.registrationSubmit": "Registrarse",
  "oryCard.emailLabel": "Correo electrónico",
  "oryCard.continue": "Continuar",
};

export default es;
