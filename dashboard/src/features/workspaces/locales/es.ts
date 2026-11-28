import type { TranslationEntry } from "@/i18n";

const esWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "ID del workspace copiado",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "No se pudo copiar el ID del workspace",
    description: "Toast when copying the workspace ID fails",
  },
  "workspaces.switcherEmpty": {
    message: "Selecciona un espacio de trabajo",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "Espacios de trabajo",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherBilling": {
    message: "Facturación",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "Configuración del espacio de trabajo",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ Nuevo espacio de trabajo",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "Nuevo espacio de trabajo",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "Plan",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "$0/mes",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "Para particulares que están empezando.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "1 miembro",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "Hasta 25 servicios",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "5 espacios de trabajo Hobby por usuario",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/mes",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "Para equipos pequeños que lanzan juntos.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "Miembros ilimitados",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "Servicios ilimitados",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/mes por organización",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
  },
  "workspaces.planScaleDescription": {
    message: "Para equipos en crecimiento que necesitan roles adicionales.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "Miembros ilimitados",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "Servicios ilimitados",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "Roles adicionales (Colaborador, Lector, Facturación)",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "Condiciones personalizadas por organización",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
  },
  "workspaces.planEnterpriseDescription": {
    message: "Límites y soporte personalizados.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "Límites personalizados",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "Soporte personalizado",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "Seleccionar plan",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "Plan seleccionado",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "El uso de servicios y almacenes de datos se factura por separado según el nivel de cada recurso.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "Crear un espacio de trabajo",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message:
      "Elige sus detalles, el contacto de facturación, el plan y el método de pago.",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "Detalles del espacio de trabajo",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "Correo de facturación",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message:
      "Los recibos y avisos de facturación de este espacio de trabajo se envían aquí.",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message:
      "En los espacios de trabajo Hobby, el correo de facturación es el de tu cuenta.",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "Introduce un correo de facturación válido.",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "Slug del espacio de trabajo",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "Una clave única para la CLI y la API. Letras minúsculas, números y guiones, de 1 a 30 caracteres.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "Solo letras minúsculas, números y guiones, de 1 a 30 caracteres, sin guion al principio ni al final.",
    description: "Workspace slug validation error",
  },
  "workspaces.paymentTitle": {
    message: "Método de pago",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "La facturación es independiente en cada espacio de trabajo. Este método de pago pertenece solo al nuevo espacio de trabajo.",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "Este espacio de trabajo requiere un método de pago.",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "Añade un método de pago ahora o continúa sin él.",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message:
      "El cobro de pagos está desactivado en esta instalación autoalojada.",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "Añadir método de pago",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "Guardar método de pago",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "Método de pago verificado para este espacio de trabajo.",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message: "No pudimos verificar ese método de pago. Inténtalo de nuevo.",
    description: "Payment Element fallback error",
  },
  "workspaces.fieldName": {
    message: "Nombre",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "p. ej. Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "p. ej. acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "Introduce un nombre de 1 a 100 caracteres sin saltos de línea.",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "Plan",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "No se pudo crear el espacio de trabajo",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "Cancelar",
    description: "/new/workspace cancel button",
  },
  "workspaces.createCancelError": {
    message:
      "No se pudo cancelar la creación del espacio de trabajo. Inténtalo de nuevo.",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
  },
  "workspaces.createSubmit": {
    message: "Crear espacio de trabajo",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "{name} creado",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "No se pudo crear el espacio de trabajo",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.creationLoadError": {
    message:
      "No se pudo cargar la configuración del espacio de trabajo. Actualiza la página para intentarlo de nuevo.",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
  },
  "workspaces.settingsTitle": {
    message: "Configuración del espacio de trabajo",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message:
      "Cambia el nombre de este espacio de trabajo o revisa su plan y sus metadatos.",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "Ningún espacio de trabajo seleccionado.",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "Secciones de configuración",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "Guardar",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "No se pudo cambiar el nombre del espacio de trabajo",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "Nombre cambiado a {name}",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "No se pudo cambiar el nombre del espacio de trabajo",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "ID del espacio de trabajo",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "Creado",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "Zona de peligro",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "Esto eliminará todos los recursos y datos de tu espacio de trabajo. Se perderán todos los servicios, almacenes de datos y variables de entorno. Esta acción no se puede deshacer.",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "Escribe {phrase} abajo para confirmar.",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "No se pudo eliminar el espacio de trabajo",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "Eliminar espacio de trabajo",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "{name} eliminado",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "No se pudo eliminar el espacio de trabajo",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "Cambiar de plan",
    description:
      "Plan link on Workspace Settings, the plan gates and the webhooks gate, opening Render's plan page (w3/m708)",
  },
  "workspaces.generalTitle": {
    message: "General",
    description: "Title of the general card on the workspace settings page",
  },
  "workspaces.ipRulesTitle": {
    message: "Reglas de IP de entrada",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "Elige qué direcciones IP pueden acceder a todos los Web Services, Static Sites, bases de datos Postgres e instancias de Key Value de este espacio de trabajo. Estas reglas se aplican además de las reglas del entorno y del servicio, por lo que un origen debe estar permitido en cada nivel. Los Private Services, Background Workers y Cron Jobs no se ven afectados.",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "Introduce un bloque CIDR (p. ej. 203.0.113.0/24) y pulsa Añadir origen. Sin reglas, se bloquea todo el tráfico entrante; 0.0.0.0/0 y ::/0 permiten todos los orígenes IPv4 e IPv6.",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "Sin reglas: se bloquea todo el tráfico entrante a los servicios y almacenes de datos de este espacio de trabajo. Añade 0.0.0.0/0 y ::/0 para permitir todos los orígenes.",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "Abierto a todas las IP de origen",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "Las reglas de IP de entrada de un espacio de trabajo requieren el plan Scale o superior. Las reglas ya definidas siguen aplicándose y se pueden restablecer para permitir todo.",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar estas reglas.",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "El plan de este espacio de trabajo no incluye reglas de IP de entrada. Mejora a Scale o superior para restringir el tráfico; siempre puedes restablecer las reglas para permitir todo.",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message:
      "No se pudieron cargar las reglas de IP de entrada de este espacio de trabajo.",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "Restablecer para permitir todo",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "Nuevo bloque CIDR",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "Bloque CIDR de la regla {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "Descripción de la nueva regla",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "Descripción de la regla {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "Descripción (opcional)",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "Añadir origen",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "Cancelar",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "Guardar reglas",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "Introduce bloques CIDR IPv4 o IPv6 válidos y únicos.",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "Quitar {cidr}",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "Subir {cidr}",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "Bajar {cidr}",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "Reglas de IP de entrada guardadas",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "No se pudieron guardar las reglas de IP de entrada",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message:
      "No se pudieron guardar las reglas de IP de entrada. Inténtalo de nuevo.",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "Autenticación de dos factores",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "Los miembros sin autenticación de dos factores (2FA) no pueden acceder a los recursos ni a la configuración de este espacio de trabajo hasta que la activen. Sus claves de API y claves SSH también quedan bloqueadas hasta entonces.",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "Exigir autenticación de dos factores (2FA)",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.loginRequirementStatusRequired": {
    message: "Requerida",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "No requerida",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.loginRequirementAdminOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar este ajuste.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "Exigir la autenticación de dos factores requiere el plan Pro o superior. Un espacio de trabajo que ya la exige la mantiene hasta que un administrador la desactive.",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "Ahora se requiere autenticación de dos factores en {name}",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "Ya no se requiere autenticación de dos factores en {name}",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "No se pudo cambiar el ajuste de dos factores",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message:
      "No se pudo cambiar el ajuste de dos factores. Inténtalo de nuevo.",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "Activa la autenticación de dos factores en tu propia cuenta antes de exigirla en este espacio de trabajo, o te quedarías fuera.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message:
      "La exigencia de dos factores no está disponible en esta instalación, así que no se puede activar.",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "Configurar la autenticación de dos factores",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} requiere autenticación de dos factores",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "Un administrador de este espacio de trabajo exige que todos los miembros usen autenticación de dos factores (2FA). Hasta que la actives, no puedes acceder a los recursos ni a la configuración de este espacio de trabajo, y tus claves de API y claves SSH no se pueden usar con él.",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "Ya la activé, comprobar de nuevo",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message:
      "Después de activar 2FA, puede tardar hasta un minuto en volver el acceso.",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.loginBlockedSwitchLabel": {
    message: "Cambiar a otro espacio de trabajo",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "Elige un espacio de trabajo",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "No hay otros espacios de trabajo",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Inicio de sesión con Google",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "Los miembros que no inician sesión con la cuenta de Google de su correo electrónico de bex no pueden acceder a los recursos ni a la configuración de este espacio de trabajo. Las claves de API y claves SSH creadas con otro inicio de sesión también quedan bloqueadas.",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "Exigir inicio de sesión con Google",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "Exigir el inicio de sesión con Google requiere el plan Pro o superior. Un espacio de trabajo que ya lo exige lo mantiene hasta que un administrador lo desactive.",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "Ahora se requiere iniciar sesión con Google en {name}",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "Ya no se requiere iniciar sesión con Google en {name}",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message: "No se pudo cambiar el ajuste de inicio de sesión con Google",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message:
      "No se pudo cambiar el ajuste de inicio de sesión con Google. Inténtalo de nuevo.",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "Inicia sesión con la cuenta de Google de tu correo electrónico de bex antes de exigirlo en este espacio de trabajo, o te quedarías fuera.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message:
      "La exigencia de inicio de sesión con Google no está disponible en esta instalación, así que no se puede activar.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "Iniciar sesión con Google",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message:
      "No se pudo empezar a iniciar sesión con Google. Inténtalo de nuevo.",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} requiere iniciar sesión con Google",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "Un administrador de este espacio de trabajo exige que todos los miembros inicien sesión con Google. Hasta que inicies sesión con la cuenta de Google de tu correo electrónico de bex, no puedes acceder a los recursos ni a la configuración de este espacio de trabajo, y las claves de API y claves SSH que creaste con otro inicio de sesión no se pueden usar con él.",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "Volverás a esta página después de iniciar sesión.",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "¿Exigir la autenticación de dos factores en {workspace}?",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "Los miembros sin autenticación de dos factores pierden el acceso a este espacio de trabajo hasta que la activen, y sus claves de API y claves SSH dejan de funcionar en él.",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "¿Exigir el inicio de sesión con Google en {workspace}?",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "Los miembros que no han iniciado sesión con la cuenta de Google de su correo electrónico de bex pierden el acceso a este espacio de trabajo hasta que inicien sesión así, y las claves de API y claves SSH creadas con otro inicio de sesión dejan de funcionar en él.",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "Exigir",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "Quién perdería el acceso",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "Comprobando quién perdería el acceso…",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message:
      "No se pudo comprobar quién perdería el acceso. Aun así, puedes activarlo.",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "Nadie perdería el acceso: todos los miembros cumplen este requisito y ninguna clave de API ni clave SSH dejaría de funcionar.",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "Cada miembro de la lista perderá el acceso hasta que active la autenticación de dos factores. Cada clave de API y clave SSH de la lista dejará de funcionar hasta que su propietario la active.",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "Cada miembro de la lista perderá el acceso hasta que inicie sesión con la cuenta de Google de su correo electrónico de bex. Cada clave de API y clave SSH de la lista dejará de funcionar; su propietario puede crear una nueva tras iniciar sesión con Google.",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "Pierde el acceso",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "Mantiene el acceso",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "Clave de API {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "Clave de API {name} en {workspace}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "Clave SSH {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message:
      "Las claves de API sin creador registrado, o cuyo creador ya no es miembro, dejarán de funcionar:",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message:
      "A los miembros que cumplen el requisito pero han iniciado sesión de otra forma se les pedirá que inicien sesión con Google.",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.loginNoticeMembers_one": {
    message: "{count} miembro",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_many": {
    message: "{count} de miembros",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_other": {
    message: "{count} miembros",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_one": {
    message:
      "{members} de tu equipo aún no ha activado la autenticación de dos factores.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_many": {
    message:
      "{members} de tu equipo aún no han activado la autenticación de dos factores.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message:
      "{members} de tu equipo aún no han activado la autenticación de dos factores.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_one": {
    message: "{members} de tu equipo aún no ha iniciado sesión con Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_many": {
    message: "{members} de tu equipo aún no han iniciado sesión con Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message: "{members} de tu equipo aún no han iniciado sesión con Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_one": {
    message: "Ver miembro",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_many": {
    message: "Ver miembros",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "Ver miembros",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "Todos los miembros",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message:
      "¡{members} de tu equipo han activado la autenticación de dos factores!",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "¡{members} de tu equipo han iniciado sesión con Google!",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "Miembros sin autenticación de dos factores",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "Miembros sin inicio de sesión con Google",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_one": {
    message:
      "Este miembro tiene prohibido actualmente el acceso a este espacio de trabajo del equipo y a sus recursos.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_many": {
    message:
      "Estos miembros tienen prohibido actualmente el acceso a este espacio de trabajo del equipo y a sus recursos.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message:
      "Estos miembros tienen prohibido actualmente el acceso a este espacio de trabajo del equipo y a sus recursos.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_one": {
    message: "¿Recordarle a este usuario que active la 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_many": {
    message: "¿Recordarles a estos usuarios que activen la 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "¿Recordarles a estos usuarios que activen la 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_one": {
    message: "Envía un recordatorio por correo electrónico a este usuario.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_many": {
    message: "Envía recordatorios por correo electrónico a estos usuarios.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "Envía recordatorios por correo electrónico a estos usuarios.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_one": {
    message: "Enviar recordatorio",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_many": {
    message: "Enviar recordatorios",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "Enviar recordatorios",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message:
      "Lo sentimos. No pudimos enviar recordatorios a estos miembros del equipo. ¿Intentar de nuevo?",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message:
      "¡Recordatorios enviados por correo electrónico a los miembros sin autenticación de dos factores!",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "¡Se alcanzó el límite de recordatorios!",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message:
      "Parece que ya enviamos recordatorios hoy. Puedes volver a enviar recordatorios mañana.",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_one": {
    message: "Se enviará un recordatorio para activar la 2FA a este miembro.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_many": {
    message: "Se enviarán recordatorios para activar la 2FA a estos miembros.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "Se enviarán recordatorios para activar la 2FA a estos miembros.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "Política de despliegues superpuestos",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "Elige qué ocurre cuando se activa el despliegue de un servicio mientras otro despliegue suyo está en curso.",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "Esperar",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "Deja que termine el despliegue en curso y pasa directamente al último despliegue activado.",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "Reemplazar",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "Cancela de inmediato el despliegue en curso e inicia el nuevo.",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "Guardar",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "Guardando…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "Política de despliegues superpuestos guardada",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message:
      "No se pudo guardar la política de despliegues superpuestos. Inténtalo de nuevo.",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar esta política.",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
  "workspaces.orgDowngradePersonRole": {
    message: "{person} ({role})",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "este espacio de trabajo",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "el entorno {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "el servicio {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message:
      "El plan de este espacio de trabajo es el de su organización. Cámbialo en el espacio de trabajo principal de la organización.",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message:
      "Incluido en el plan de tu organización. El espacio de trabajo principal de la organización gestiona su plan y su facturación.",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "Ir al espacio de trabajo principal",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "Ir a la facturación del espacio de trabajo principal",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "Privacidad del espacio de trabajo",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message:
      "Elige quién de tu organización puede unirse a este espacio de trabajo.",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "Público",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message:
      "Cualquier miembro de tu organización puede unirse por su cuenta a este espacio de trabajo.",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "Solo por invitación",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message:
      "Solo los administradores del espacio de trabajo pueden invitar a personas a este espacio de trabajo.",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "Guardar",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "Guardando…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "Privacidad del espacio de trabajo guardada",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message:
      "No se pudo guardar la privacidad del espacio de trabajo. Inténtalo de nuevo.",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message: "Lo exige {organization} en todos sus espacios de trabajo.",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "Cámbialo en la configuración de la organización",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message:
      "{organization} requiere autenticación de dos factores en todos sus espacios de trabajo",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "Un propietario de esta organización exige que todos los miembros usen autenticación de dos factores (2FA) en la organización y en todos sus espacios de trabajo. Hasta que la actives, no puedes acceder a ellos, y tus claves de API y claves SSH no se pueden usar con ellos.",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message:
      "{organization} requiere iniciar sesión con Google en todos sus espacios de trabajo",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "Un propietario de esta organización exige que todos los miembros inicien sesión con Google en la organización y en todos sus espacios de trabajo. Hasta que inicies sesión con la cuenta de Google de tu correo electrónico de bex, no puedes acceder a ellos, y las claves de API y claves SSH que creaste con otro inicio de sesión no se pueden usar con ellos.",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.updatePlanPageTitle": {
    message: "Cambiar plan",
    description: "Document title of the plan pages (w3/m708)",
  },
  "workspaces.updatePlanTitle": {
    message: "Cambiar plan de {name}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): page title with the workspace's name",
  },
  "workspaces.updatePlanConfirmTitle": {
    message: "Cambiar plan de {name}",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): page title with the workspace's name (Render capitalizes Plan here)",
  },
  "workspaces.updatePlanBack": {
    message: "Atrás",
    description:
      "Accessible label of the plan pages' Back button, and the confirmation's Back action (w3/m708)",
  },
  "workspaces.updatePlanSelectTitle": {
    message: "Selecciona un plan",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the plan cards",
  },
  "workspaces.updatePlanCompare": {
    message: "Obtén más información o compara los planes en nuestra {link}.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under Select a plan; {link} is the updatePlanPricingLink link to bex's pricing page",
  },
  "workspaces.updatePlanPricingLink": {
    message: "página de precios",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): the link in updatePlanCompare",
  },
  "workspaces.updatePlanPaymentTitle": {
    message: "Método de pago",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the payment method section",
  },
  "workspaces.updatePlanCardRequired": {
    message:
      "Este plan de workspace requiere una tarjeta para los servicios de pago y el cómputo.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): notice while a plan other than Hobby is selected",
  },
  "workspaces.updatePlanHowBillingWorks": {
    message: "Cómo funciona la facturación",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): link beside updatePlanCardRequired to bex's billing docs",
  },
  "workspaces.updatePlanWorkspaceBillingTitle": {
    message: "Facturación del workspace",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading beside the card on file",
  },
  "workspaces.updatePlanWorkspaceBillingBody": {
    message:
      "La facturación es independiente para cada workspace. Se cobrarán a tu tarjeta las tarifas de los servicios de pago o del cómputo de este workspace.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under updatePlanWorkspaceBillingTitle",
  },
  "workspaces.updatePlanErrorsSummary": {
    message:
      "Hay algunos errores en tu selección. Revísalos y corrígelos para continuar.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): summary under the plan rules a selection breaks",
  },
  "workspaces.updatePlanErrorPrefix": {
    message: "Se produjo un error. {message}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): a refused change; {message} is the reason",
  },
  "workspaces.updatePlanPaymentRequired": {
    message: "Añade una tarjeta válida para cambiar a un plan de pago.",
    description:
      "The plan pages' message when bex-api refuses a paid plan with PAYMENT_REQUIRED (w3/m708)",
  },
  "workspaces.updatePlanServerError": {
    message: "Error del servidor al enviar. Inténtalo de nuevo",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): fallback reason when an upgrade fails without one",
  },
  "workspaces.updatePlanSubmit": {
    message: "Seleccionar plan",
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
      "Tu rol no puede cambiar el plan de este workspace. Pide ayuda a un administrador del workspace.",
    description:
      "The plan pages' notice for a member who cannot change the plan (can_manage) (w3/m708)",
  },
  "workspaces.planCurrentBadge": {
    message: "Plan actual",
    description:
      "Badge on the workspace's own plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planNewBadge": {
    message: "Nuevo plan",
    description:
      "Badge on the chosen plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planContact": {
    message: "Contactar",
    description:
      "Enterprise card's contact action on the plan page, Render's Get in touch (w3/m708)",
  },
  "workspaces.updatePlanSummaryTitle": {
    message: "Resumen del nuevo plan",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the plan comparison and the card on file",
  },
  "workspaces.updatePlanConfirmDowngradeTitle": {
    message: "Confirmar el cambio a un plan inferior",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the typed confirmation a downgrade asks for",
  },
  "workspaces.updatePlanConfirmChangesIncluding": {
    message: "Para iniciar los cambios en tu plan, entre ellos:",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): lead-in above the list of what a downgrade changes",
  },
  "workspaces.updatePlanConfirmTypeAfterList": {
    message:
      "Escribe {phrase} en el cuadro de texto y haz clic en el botón de confirmar.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase after the list of changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmType": {
    message:
      "Para iniciar los cambios en tu plan, escribe {phrase} en el cuadro de texto y haz clic en el botón de confirmar.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase when the downgrade lists no changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmSubmit": {
    message: "Confirmar",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): confirm button",
  },
  "workspaces.updatePlanErrorTitle": {
    message: "Se produjo un error.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): title of a refused change",
  },
  "workspaces.updatePlanError": {
    message: "Error al actualizar el plan.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): fallback reason when the change fails without one",
  },
  "workspaces.updatePlanRemovesOrganization": {
    message: "La organización se eliminará.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): bold start of the change an org primary's downgrade below Scale makes (ADR115 D6)",
  },
  "workspaces.updatePlanRemovesOrganizationDetail": {
    message:
      "Sus funciones de nivel de organización terminan y este workspace continúa como workspace independiente.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): rest of the org removal line after updatePlanRemovesOrganization",
  },
  "workspaces.planUpdated": {
    message: "Tu workspace ahora tiene el plan {plan}.",
    description:
      "Toast after a plan change, Render's plan-updated toast; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planUpdatedWelcome": {
    message: "Bienvenido al plan {plan}.",
    description:
      "Toast after a change to Scale, Render's welcome; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planComparisonCurrent": {
    message: "Plan actual",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the current plan's side of the comparison",
  },
  "workspaces.planComparisonNew": {
    message: "Nuevo plan",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the new plan's side of the comparison",
  },
  "workspaces.planComparisonHighlights": {
    message: "Aspectos destacados",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): label above each side's highlights (shown in capitals)",
  },
  "workspaces.planHighlightJustForMe": {
    message: "Solo para mí",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's one seat",
  },
  "workspaces.planHighlightUnlimitedMembers": {
    message: "Miembros del equipo ilimitados",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited seats",
  },
  "workspaces.planHighlightIncludedDomains": {
    message:
      "{included} dominios personalizados incluidos, luego {price}/dominio",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): custom domains included before they are metered; {included} is the count, {price} bex's price per domain (ADR030 §16)",
  },
  "workspaces.planHighlightUnlimitedDomains": {
    message: "Dominios personalizados ilimitados",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): every custom domain included (Enterprise)",
  },
  "workspaces.planHighlightTwoEnvironments": {
    message: "2 entornos por proyecto",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's environments per project",
  },
  "workspaces.planHighlightUnlimitedEnvironments": {
    message: "Proyectos y entornos ilimitados",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited projects and environments",
  },
  "workspaces.planHighlightRetention": {
    message: "Ventana de {days} días para registros y métricas",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): how far back logs and metrics reach; {days} is the number of days",
  },
  "workspaces.planHighlightBasicLogs": {
    message: "Registros y métricas básicos",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's logs and metrics, without request logs",
  },
  "workspaces.planHighlightRequestLogs": {
    message: "Registros de solicitudes HTTP y métricas de tiempo de respuesta",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Pro's HTTP request logs and response time metrics",
  },
  "workspaces.planHighlightKeptBuilds": {
    message: "{builds} builds conservados para reversión instantánea",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): builds kept for rollback; {builds} is the count (core.KeptBuilds)",
  },
  "workspaces.planHighlightAuditLogs": {
    message: "Registros de auditoría del workspace",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): workspace audit logs (Pro and higher)",
  },
  "workspaces.planHighlightOrganization": {
    message:
      "Una organización, con más workspaces, roles adicionales y reglas de IP entrantes",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): what an organization adds (Scale and higher, ADR115)",
  },
  "workspaces.planBlockerMemberLimitTitle": {
    message: "Demasiados usuarios en el equipo.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more people than the plan seats",
  },
  "workspaces.planBlockerMemberLimitOne": {
    message: "Los workspaces {plan} solo pueden tener 1 usuario.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with one seat; {plan} is the plan's name",
  },
  "workspaces.planBlockerMemberLimitMany": {
    message: "Los workspaces {plan} solo pueden tener hasta {limit} usuarios.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with several seats; {plan} is the plan's name, {limit} its seats",
  },
  "workspaces.planBlockerUpdateTeamMembers": {
    message: "Actualizar miembros del equipo",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' Team section",
  },
  "workspaces.planBlockerServiceLimitTitle": {
    message: "Demasiados servicios en el workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more services than the plan allows",
  },
  "workspaces.planBlockerServiceLimit": {
    message: "Los workspaces {plan} solo pueden tener hasta {limit} servicios.",
    description:
      "Render's plan rule on its plan page (w3/m708): the plan's service cap; {plan} is the plan's name, {limit} its cap",
  },
  "workspaces.planBlockerUpdateServices": {
    message: "Actualizar servicios",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the workspace's services",
  },
  "workspaces.planBlockerWorkspaceLimitTitle": {
    message: "Demasiados workspaces Hobby.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the caller already has the most Hobby workspaces allowed",
  },
  "workspaces.planBlockerWorkspaceLimit": {
    message:
      "No puedes tener más de {limit} workspaces Hobby. Cambia a un plan superior o elimina otro workspace Hobby.",
    description:
      "Render's plan rule on its plan page (w3/m708): the cap on a person's Hobby workspaces; {limit} is the cap",
  },
  "workspaces.planBlockerWorkspaceLimitsDocs": {
    message: "Límites de workspace",
    description:
      "Render's plan rule on its plan page (w3/m708): link to bex's docs on workspace limits",
  },
  "workspaces.planBlockerWorkspacesTitle": {
    message: "Demasiados workspaces.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when an org primary's downgrade leaves secondary workspaces",
  },
  "workspaces.planBlockerWorkspaces": {
    message: "{plan} no admite más de un workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan without an organization; {plan} is the plan's name",
  },
  "workspaces.planBlockerUpdateWorkspaces": {
    message: "Actualizar workspaces",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the organization's workspaces",
  },
  "workspaces.planBlockerRolesTitle": {
    message:
      "No se puede cambiar a un plan inferior mientras se usan roles no disponibles",
    description:
      "Render's plan rule on its plan page (w3/m708): title when members or invites hold roles the plan does not offer",
  },
  "workspaces.planBlockerRolesOne": {
    message:
      "{plan} no admite el rol {roles}. Para cambiar tu workspace a un plan inferior, primero cambia los miembros del equipo y las invitaciones pendientes afectados a Developer o Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): one role the plan lacks; {plan} is the plan's name, {roles} the role",
  },
  "workspaces.planBlockerRolesMany": {
    message:
      "{plan} no admite los roles {roles}. Para cambiar tu workspace a un plan inferior, primero cambia los miembros del equipo y las invitaciones pendientes afectados a Developer o Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): several roles the plan lacks; {plan} is the plan's name, {roles} the roles",
  },
  "workspaces.planBlockerIpRulesTitle": {
    message:
      "Las listas de IP permitidas deben eliminarse antes de cambiar a un plan inferior.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when inbound IP rules stand in the way (Render's words for them on its confirmation)",
  },
  "workspaces.planBlockerIpRules": {
    message:
      "Elimina todas las configuraciones de listas de IP permitidas de tu workspace, entornos y servicios antes de cambiar de plan.",
    description:
      "Render's plan rule on its plan page (w3/m708): what to remove for planBlockerIpRulesTitle",
  },
  "workspaces.planBlockerUpdateIpRules": {
    message: "Actualizar reglas de IP entrantes",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' inbound IP rules",
  },
  "workspaces.planBlockerOtherTitle": {
    message: "Este plan aún no se puede seleccionar.",
    description:
      "Render's plan rule on its plan page (w3/m708): title of a refusal kind this dashboard does not know yet",
  },
  "workspaces.planBlockerOther": {
    message: "Este workspace aún no cumple los límites del plan {plan}.",
    description:
      "Render's plan rule on its plan page (w3/m708): line under planBlockerOtherTitle; {plan} is the plan's name",
  },
};

export default esWorkspaces;
