import type { TranslationEntry } from "@/i18n";

const esLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "Términos",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "Términos de uso de los modelos del agente",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "En vigor desde el {date}",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "Alcance",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "Estos términos se aplican cuando las sesiones de agente de un espacio de trabajo usan el modelo proporcionado por bex en lugar de la clave de proveedor de modelos propia del espacio de trabajo.",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "Tus datos",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "Para generar respuestas, bex envía los prompts, el código y la salida de herramientas de la sesión a proveedores de modelos de terceros a través de OpenRouter.",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "Las solicitudes se enrutan solo a proveedores que no almacenan los prompts más allá de su procesamiento y no entrenan con ellos: bex usa el enrutamiento de proveedores de OpenRouter con la recopilación de datos denegada.",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Las sesiones de Muse Code son la excepción. bex las envía directamente a la API de modelos de Meta en su nivel de colaborador, bajo el cual Meta puede usar los prompts, el código, los archivos y el contexto enviados, y las respuestas devueltas, para entrenar sus modelos. Muse Code permanece desactivado en un espacio de trabajo hasta que un administrador lo activa y reconoce esto.",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "Si el espacio de trabajo usa en su lugar su propia clave de proveedor, las solicitudes van a tu proveedor conforme a tu acuerdo con él, y bex no factura tokens de modelo.",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "Precio",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "Cada solicitud se factura al coste real del proveedor del modelo más un 30 %. El uso se mide por solicitud y se muestra en Facturación → Modelo del agente.",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "Límites de gasto",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "Cada espacio de trabajo tiene un límite de gasto mensual según su plan: Hobby $20, Pro $200, Scale $2,000. Los administradores del espacio de trabajo pueden ajustarlo hasta 10 veces el límite del plan.",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "Un espacio de trabajo nuevo puede ejecutar dos sesiones simultáneas con el modelo proporcionado por bex hasta su primer pago correcto.",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "Cuando se alcanza el límite, las nuevas solicitudes al modelo se rechazan hasta que comience el mes siguiente o un administrador aumente el límite.",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "Cargos durante el mes",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "Autorizas a bex a cobrar al método de pago del espacio de trabajo cada vez que su uso no facturado del modelo del agente alcance el umbral de cobro del espacio de trabajo, además de la factura mensual. El espacio de trabajo sigue funcionando mientras se realiza el cargo. El umbral empieza en $10 y aumenta con el historial de pagos del espacio de trabajo.",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "Si un cargo falla o requiere tu autenticación, el uso del modelo del agente se pausa hasta que se pague el saldo.",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "Eliminar un espacio de trabajo o una cuenta",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "Antes de eliminar un espacio de trabajo o una cuenta, bex cobra su uso no facturado. Si ese cargo falla, la eliminación espera hasta que se pague el saldo.",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "Autorizas a bex a cobrar el uso incurrido antes de la eliminación, incluso después de ella.",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "Métodos de pago",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "No se aceptan tarjetas de prepago. Una tarjeta vinculada a un saldo pendiente de bex puede ser rechazada.",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "Registros de uso",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex conserva un registro de cada solicitud (hora, modelo, tokens y precio) durante 13 meses. Estos registros pueden usarse como prueba si se disputa un cargo.",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "Cambios en estos términos",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "bex puede actualizar estos términos. La fecha de entrada en vigor que aparece al principio de esta página indica la versión actual.",
    description: "Terms: changes",
  },
};

export default esLegal;
