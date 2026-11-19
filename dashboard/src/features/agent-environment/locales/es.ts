import type { TranslationEntry } from "@/i18n";

const esAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "Agentes",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "Cómo acceden a la red las sesiones de agente de este espacio de trabajo y qué modelo usan. Se aplica a todas las sesiones nuevas.",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "No se pudo cargar la configuración de los agentes.",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "Reintentar",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar la configuración de los agentes.",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "Acceso a la red",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "GitHub y el proveedor del modelo siempre son accesibles. Elige a qué más puede acceder el sandbox del agente.",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "Registros de paquetes (recomendado)",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "El agente puede instalar dependencias desde npm, PyPI, Go, Maven, crates, RubyGems y Docker Hub, para poder compilar y probar tu código.",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "Personalizado",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message:
      "Los registros de paquetes más los nombres de host que indiques abajo.",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "Internet completo",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "Cualquier sitio web público por HTTP(S) a través del proxy de salida de bex. Las llamadas directas a las API de proveedores de modelos siguen bloqueadas (el modelo propio del agente sigue funcionando), y SSH y otros protocolos que no son HTTP no están disponibles.",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_one": {
    message: "Registros de paquetes incluidos ({count} host)",
    description: "Disclosure summary listing the built-in registry hosts (one)",
  },
  "agentEnvironment.registryHostsSummary_many": {
    message: "Registros de paquetes incluidos ({count} de hosts)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "Registros de paquetes incluidos ({count} hosts)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "Nombres de host adicionales",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "Un nombre de host por línea, hasta {max}. Solo nombres exactos: sin esquema, puerto, ruta ni comodín.",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "Indica como máximo {max} nombres de host.",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "Los nombres de host no pueden estar vacíos.",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: "“{entry}” tiene espacios alrededor.",
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      "“{entry}” no es un nombre de host exacto. Usa un nombre como api.example.com, sin esquema, puerto, ruta ni comodín.",
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message: "“{entry}” es una dirección IP. Usa un nombre de host.",
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      "“{entry}” es un nombre privado o local del clúster y no se puede permitir.",
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      "“{entry}” ya es accesible (o solo es accesible a través del proxy del modelo). Quítalo de la lista.",
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: "“{entry}” aparece más de una vez.",
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "La lista de nombres de host no es válida.",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "Modelo predeterminado",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "Predeterminado del proveedor",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "ID del modelo que usan las sesiones nuevas, como claude-sonnet-4-5. Déjalo vacío para usar el predeterminado del proveedor.",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "Predeterminado de la plataforma ({model})",
    description: "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "Las sesiones y automatizaciones nuevas que no indican un modelo usan este. Los miembros aún pueden elegir otro modelo del catálogo de la plataforma para una sesión.",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message: "Usa un ID de modelo sin espacios (máximo 255 caracteres).",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message:
      "El modelo de la plataforma no ofrece {model}. Elige uno de la lista.",
    description: "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.tierInvalid": {
    message: "Esa opción de acceso a la red no es compatible.",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar la configuración de los agentes.",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message:
      "No se pudo guardar la configuración de los agentes. Inténtalo de nuevo.",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "No se pudo guardar la configuración de los agentes: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "Guardar",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "Configuración de los agentes guardada",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "Proveedor del modelo",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "Las sesiones de agente usan el modelo que proporciona bex, salvo que el espacio de trabajo añada su propia clave de proveedor. Con tu propia clave, las sesiones llaman directamente a tu proveedor y bex solo factura el cómputo del sandbox.",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "Origen del modelo:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message:
      "Proporcionado por bex (se factura al coste del proveedor del modelo + 30 %)",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "Tu propia clave (bex no cobra tokens)",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message:
      "Ninguno: añade una clave de proveedor para iniciar sesiones de agente",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "Clave de API del proveedor",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "Reemplazar la clave de API del proveedor",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "Hay una clave guardada; introduce una nueva para reemplazarla",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message:
      "Solo escritura: la clave se almacena de forma segura y no se vuelve a mostrar.",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "Guardar clave",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "Quitar clave",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "¿Quitar la clave del proveedor del modelo?",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "Las sesiones de agente dejan de usar esta clave a partir de su siguiente solicitud al modelo. La clave no se puede volver a mostrar, así que tendrías que pegarla para restaurarla.",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "Quitar clave",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "Clave del modelo guardada",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "Clave del modelo quitada",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "Introduce la clave sin espacios (máximo 4096 caracteres).",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "No se pudo guardar la clave del modelo. Inténtalo de nuevo.",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "Gasto del modelo del agente",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "Se factura al coste del proveedor del modelo + 30 %. El límite detiene las nuevas solicitudes al modelo durante el resto del mes.",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "{used} de {cap} este mes",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message:
      "{used} de {cap} más allá de los créditos de facturación, {total} este mes",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{used} de {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "El gasto no está disponible en este momento.",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "Nivel de confianza {tier}: los créditos de facturación pagan primero el gasto del modelo del agente; se cobra a tu tarjeta cuando el gasto que dejan sin cubrir alcanza {amount}.",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "Nivel de confianza {tier}: los créditos de facturación cubren los primeros {credit} de gasto del modelo del agente en este período de facturación; se cobra a tu tarjeta cuando el gasto que los supera alcanza {amount}.",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message:
      "Nivel de confianza {tier}: el gasto del modelo del agente se cobra a final de mes.",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "A los espacios de trabajo nuevos se les cobra cuando el gasto del modelo del agente, más allá de los créditos de facturación, alcanza {amount}: la tarjeta se cobra en ese momento, las sesiones siguen ejecutándose en cuanto el cobro se realiza y el límite sube al tope de tu plan.",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "Un pago fallido o disputado limita este espacio de trabajo; resuélvelo en Facturación para quitar el límite.",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "Límite de gasto mensual (USD)",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "Guardar límite",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "Restablecer al predeterminado del plan",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message:
      "Predeterminado del plan: {plan}. Los administradores pueden establecer hasta {max}.",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "Introduce una cantidad mayor que $0 y de {max} como máximo.",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "Límite de gasto guardado",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "Límite de gasto restablecido al predeterminado del plan",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "No se pudo guardar el límite de gasto. Inténtalo de nuevo.",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "Secretos de agente",
    description: "Settings → Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "Claves de API y tokens que se pueden entregar a una sesión como variables de entorno. Los valores son de solo escritura: nadie puede volver a leerlos y se ocultan en las transcripciones.",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "No se pudieron cargar los secretos de agente.",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "Aún no hay secretos de agente.",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "Eliminar {name}",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "Las sesiones que usan este secreto fallan en su siguiente turno hasta que se vuelva a establecer.",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "Eliminar secreto",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "Nombre del secreto",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "Valor del secreto",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "Valor",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "Guardar",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "Guardar un nombre existente reemplaza su valor. Las sesiones recogen los cambios a partir de su siguiente turno.",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden añadir, cambiar o eliminar secretos de agente.",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "{name} guardado",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "{name} eliminado",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message:
      "Usa letras, dígitos y _, empezando por una letra o _ (máximo 128).",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "bex establece esta variable de entorno por sí mismo, así que un secreto no puede usar ese nombre.",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "Introduce un valor.",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "Un valor puede tener como máximo 32 KiB.",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "Se alcanzó el límite de secretos de agente.",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message:
      "Este secreto ya no existe; puede que alguien lo haya eliminado hace un momento.",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "Los secretos de agente no están disponibles en este despliegue.",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar los secretos de agente.",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "No se pudo guardar el cambio. Inténtalo de nuevo.",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "Mantener el valor actual",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "Editar {name}",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "Ámbito",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "Todas las sesiones",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "Un repositorio",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "Solo cuando se selecciona",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "Repositorio: {repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "Repositorio",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "Ningún repositorio conectado coincide.",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "Introduce el repositorio como owner/name.",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "Entornos de repositorio",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "Cómo preparan las sesiones de agente cada repositorio. Los pasos de configuración se ejecutan en orden desde la raíz del repositorio antes de que arranque el agente de cada sesión, con los secretos de la sesión. Las comprobaciones son los comandos que el agente ejecuta antes de entregar, como lint y test.",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "No se pudieron cargar los entornos de repositorio.",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "Aún no hay entornos de repositorio.",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "Configuración: {names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "Comprobaciones: {names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "Sin pasos de configuración",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "Sin comprobaciones",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "Editar {repo}",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "Ver {repo}",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "Eliminar el entorno de {repo}",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "¿Eliminar el entorno de {repo}?",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message:
      "A partir de su siguiente turno, las sesiones de este repositorio arrancan sin sus pasos de configuración ni comprobaciones.",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "Eliminar entorno",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "Añadir entorno",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message:
      "Al guardar se reemplaza todo el entorno del repositorio. Las sesiones recogen los cambios a partir de su siguiente turno.",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden añadir, cambiar o eliminar entornos de repositorio.",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "Entorno de {repo} guardado",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "Entorno de {repo} eliminado",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "Añadir entorno de repositorio",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "Editar {repo}",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "Los pasos de configuración se ejecutan en orden desde la raíz del repositorio antes de que arranque el agente de cada sesión, con los secretos de la sesión. Si un paso falla, el turno termina antes de que arranque el agente. Las comprobaciones son lo que el agente ejecuta antes de entregar.",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "Repositorio",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "Ningún repositorio conectado coincide.",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "Pasos de configuración",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "Instala herramientas y dependencias. Cada paso se ejecuta con bash, en orden; máximo 10 pasos, de 10 minutos cada uno.",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "Comprobaciones",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "Comandos como lint, test y build. El agente ejecuta cada uno desde la raíz del repositorio antes de entregar e informa de los resultados; bex no los ejecuta por sí mismo. Máximo 10.",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "Añadir paso de configuración",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "Añadir comprobación",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "Nombre del paso de configuración {position}",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "Comando del paso de configuración {position}",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "Subir el paso de configuración {position}",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "Bajar el paso de configuración {position}",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "Quitar el paso de configuración {position}",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "Nombre de la comprobación {position}",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "Comando de la comprobación {position}",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "Subir la comprobación {position}",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "Bajar la comprobación {position}",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "Quitar la comprobación {position}",
    description: "Accessible label: remove a check row",
  },
  "agentEnvironment.repoEnvs.setupNamePlaceholder": {
    message: "deps",
    description: "Setup step name placeholder (an example step name)",
  },
  "agentEnvironment.repoEnvs.setupRunPlaceholder": {
    message: "npm ci",
    description: "Setup step command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.checkNamePlaceholder": {
    message: "test",
    description: "Check name placeholder (an example check name)",
  },
  "agentEnvironment.repoEnvs.checkRunPlaceholder": {
    message: "npm test",
    description: "Check command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.close": {
    message: "Cerrar",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "Cancelar",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "Guardar",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "Introduce un nombre.",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "Un nombre empieza por una letra o un dígito y usa letras, dígitos, espacios, '.', '_' o '-' (máximo 64).",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message: "Otra fila de esta lista ya usa este nombre.",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "Introduce un comando.",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "Un comando de configuración puede tener como máximo 8 KiB.",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "Un comando de comprobación puede tener como máximo 2 KiB.",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "Un entorno puede tener como máximo 10 pasos de configuración.",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "Un entorno puede tener como máximo 10 comprobaciones.",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message:
      "Añade al menos un paso de configuración o una comprobación. Para quitarlos todos, elimina el entorno.",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "Introduce el repositorio como owner/name.",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message: "Este repositorio ya tiene un entorno. Edítalo desde la lista.",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "Se rechazó un paso de configuración. Comprueba que cada nombre sea válido y único y que cada comando tenga como máximo 8 KiB.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "Se rechazó una comprobación. Comprueba que cada nombre sea válido y único y que cada comando tenga como máximo 2 KiB.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message:
      "Se rechazó este entorno. Comprueba su repositorio, sus pasos de configuración y sus comprobaciones.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message:
      "Este espacio de trabajo alcanzó su límite de 50 entornos de repositorio. Elimina uno para añadir otro.",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message:
      "Este repositorio ya no tiene un entorno; puede que alguien lo haya eliminado hace un momento.",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message:
      "Los entornos de repositorio no están disponibles en este despliegue.",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar los entornos de repositorio.",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message: "No se pudo guardar el cambio. Inténtalo de nuevo.",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "En cola",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "Compilando",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "Listo",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "Fallido",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "Cancelado",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "Cambiaron los pasos de configuración",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "Cambió un secreto del repositorio",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "Build manual",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "Actualización diaria",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "Falló un paso de configuración.",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "Un paso de configuración superó su límite de 3 horas.",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "El build superó su límite de 3 horas.",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message:
      "No se pudo hacer checkout de la rama predeterminada del repositorio.",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message:
      "La instantánea superaría el límite de almacenamiento de sandbox de este espacio de trabajo.",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message: "bex no pudo terminar el build por su parte. Inténtalo de nuevo.",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "El build falló.",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "Builds de {repo}",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "Builds de {repo}",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "Las sesiones arrancan desde el build correcto más reciente cuyos pasos de configuración coinciden con los actuales. Si ninguno coincide, ejecutan los pasos de configuración al inicio de cada turno.",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "Hacer build ahora",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "Los archivos que escriben los pasos de configuración se conservan en la instantánea, así que no escribas credenciales en archivos. Los secretos llegan a los pasos como variables de entorno, que no se conservan.",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden iniciar un build.",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message:
      "Solo los administradores del espacio de trabajo pueden iniciar un build.",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message:
      "Este entorno no tiene pasos de configuración, así que no hay nada que compilar. Las comprobaciones no necesitan un build.",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "Build de {repo} en cola",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message:
      "Este repositorio ya inició 12 builds en las últimas 24 horas. Inténtalo de nuevo más tarde.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "Este repositorio ya inició 12 builds en las últimas 24 horas. El siguiente build puede empezar a las {time}.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "Los builds de entorno no están disponibles en este despliegue.",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "No se pudo iniciar el build. Inténtalo de nuevo.",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "No se pudieron cargar los builds.",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "Aún no hay builds.",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "Este repositorio aún no tiene builds. Hacer build ahora ejecuta sus pasos de configuración una vez y guarda el resultado como instantánea.",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "Historial de builds",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "Log del build",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "No se pudo cargar el log de este build.",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "tardó {duration}",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "instantánea eliminada",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "Aún sin build",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message:
      "Este build está esperando para empezar. Los builds comparten con las sesiones el límite de sandboxes de agente del espacio de trabajo.",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message:
      "Los pasos de configuración se están ejecutando. Su log aparecerá aquí cuando termine el build.",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message:
      "Este build se canceló: lo reemplazó un build más reciente o se eliminó su entorno.",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "No se ejecutó ningún paso de configuración.",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "Este build no registró ningún paso.",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "salida {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "detenido",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "Sin salida.",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "Skills de agente",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "Instrucciones reutilizables que recibe cada sesión de agente de este espacio de trabajo, como archivos SKILL.md. El agente aplica un skill cuando encaja con la tarea, o solo cuando se le pide.",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "No se pudieron cargar los skills de agente.",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "Aún no hay skills de agente.",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "Solo cuando se pide",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "Habilitar {name}",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "Editar {name}",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "Ver {name}",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "Descargar {name} como SKILL.md",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "Eliminar {name}",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "¿Eliminar el skill {name}?",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "Los nuevos turnos del agente dejan de recibirlo. Las sesiones que ya se ejecutaron conservan su transcripción.",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "Eliminar skill",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "Nuevo skill",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "Importar SKILL.md",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message:
      "Los miembros pueden usar skills; solo los administradores pueden añadirlos o cambiarlos.",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message:
      "Solo los administradores del espacio de trabajo pueden añadir o cambiar skills de agente.",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "Nuevo skill",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "Editar {name}",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "El agente lee la descripción para decidir cuándo se aplica el skill y sigue las instrucciones cuando se aplica.",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "Nombre",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "Descripción",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "Cuándo usar este skill, en una línea",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "Solo cuando yo lo pida",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message: "Invócalo desde el compositor o con @skills:name en un prompt.",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "Instrucciones",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "Pasos que sigue el agente, en Markdown.",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "Editar como SKILL.md",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "Editar campos",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "Historial de versiones",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "Editar",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "Actual",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "Cambios",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "Contenido",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "Cargar en el editor",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "Cargar más",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "Esta versión no cambió nada.",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "No se pudo cargar el historial de versiones.",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "No se pudo cargar esta versión.",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message:
      "Se cargó la versión {version}. Guarda para conservarla como una versión nueva.",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "Vista previa",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "Guardar skill",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "Cancelar",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "Cerrar",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "{name} guardado",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "{name} eliminado",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "No se pudo abrir el skill. Inténtalo de nuevo.",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "No se pudo leer ese archivo.",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "Introduce un nombre.",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message:
      "Usa letras minúsculas, dígitos y guiones simples, hasta 64 caracteres.",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "Introduce una descripción de una línea.",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message:
      "Mantén la descripción en una sola línea de 1024 caracteres como máximo.",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "Las instrucciones están limitadas a 64 KiB.",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message:
      "El skill no es válido. Comprueba su nombre, descripción e instrucciones.",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "Se alcanzó un límite de skills de agente (100 por espacio de trabajo, 1 MiB habilitado en total).",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "Este skill cambió desde que lo abriste, o el nombre ya está en uso. Ciérralo y vuelve a abrirlo para ver lo más reciente.",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "Ese skill ya no existe.",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "Los skills de agente no están disponibles en este despliegue.",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message:
      "Solo los administradores del espacio de trabajo pueden cambiar los skills de agente.",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "No se pudo guardar el cambio. Inténtalo de nuevo.",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "Agentes disponibles",
    description: "Settings → Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "Elige con qué agentes pueden iniciar sesiones los miembros. Las sesiones en curso siguen ejecutándose cuando desactivas un agente.",
    description: "Settings → Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message:
      "Su proveedor de modelos puede entrenar con lo que envían sus sesiones.",
    description:
      "Settings → Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "¿Activar {agent}?",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "Este agente se ejecuta en el nivel de colaboradores de Meta. Meta puede usar los prompts, el código, los archivos y el contexto que envían sus sesiones, y las respuestas que reciben, para entrenar sus modelos. Al activarlo se registra que tú, como administrador, lo reconoces en nombre del espacio de trabajo.",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "Reconocer y activar",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "Agentes disponibles actualizados",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message:
      "No se pudieron actualizar los agentes disponibles. Inténtalo de nuevo.",
    description: "Error after a failed availability change",
  },
};

export default esAgentEnvironment;
