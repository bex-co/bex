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
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "Cambiar de plan",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "Elige un nuevo plan para este espacio de trabajo. No hay paso de pago: el plan cambia de inmediato.",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "Cancelar",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "Cambiar plan",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "No se pudo cambiar el plan",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "Plan cambiado a {plan}",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "No se pudo cambiar el plan del espacio de trabajo",
    description: "Fallback toast/inline message on a failed plan change",
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
  "workspaces.twoFactorStatusRequired": {
    message: "Requerida",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "No requerida",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
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
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "Cambiar a otro espacio de trabajo",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "Elige un espacio de trabajo",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "No hay otros espacios de trabajo",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
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
  "workspaces.orgDowngradeWarningTitle": {
    message: "Bajar a un plan inferior a Scale elimina la organización",
    description:
      "Change-plan dialog on an org's primary: title of the warning shown when a plan below Scale is selected (w7/m464)",
  },
  "workspaces.orgDowngradeWarning": {
    message:
      "Este es el espacio de trabajo principal de tu organización, y su plan cubre todos los espacios de trabajo de la organización. Al bajar de plan se elimina la organización y el acceso a sus funciones, y este espacio de trabajo sigue como espacio de trabajo independiente. Primero elimina los demás espacios de trabajo de la organización, cambia a Desarrollador o Administrador los miembros e invitaciones pendientes con el rol Facturación, Lector o Colaborador, y elimina las reglas de IP de entrada de los servicios, los entornos y este espacio de trabajo.",
    description:
      "Change-plan dialog on an org's primary: what a downgrade below Scale does and Render's preconditions (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.orgDowngradeBlocked": {
    message:
      "Todavía no se puede eliminar la organización. Antes de bajar a un plan inferior a Scale:",
    description:
      "Change-plan dialog: lead-in above the unmet preconditions of ORGANIZATION_DOWNGRADE_BLOCKED (w7/m464)",
  },
  "workspaces.orgDowngradeSecondaryWorkspaces": {
    message: "Elimina los demás espacios de trabajo de la organización",
    description:
      "Unmet downgrade precondition secondary_workspaces; the workspace names follow (w7/m464)",
  },
  "workspaces.orgDowngradeMemberRoles": {
    message: "Cambia el rol de estos miembros a Desarrollador o Administrador",
    description:
      "Unmet downgrade precondition member_roles; the members and roles follow (w7/m464)",
  },
  "workspaces.orgDowngradeInviteRoles": {
    message:
      "Cambia el rol de estas invitaciones pendientes a Desarrollador o Administrador",
    description:
      "Unmet downgrade precondition invite_roles; the invited emails and roles follow (w7/m464)",
  },
  "workspaces.orgDowngradeInboundIpRules": {
    message: "Elimina las reglas de IP de entrada",
    description:
      "Unmet downgrade precondition inbound_ip_rules; the workspace, environments and services that hold them follow (w7/m464)",
  },
  "workspaces.orgDowngradeItems": {
    message: "{label}: {items}",
    description:
      "One unmet downgrade precondition: {label} is what to do, {items} the list of what it applies to (w7/m464)",
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
};

export default esWorkspaces;
