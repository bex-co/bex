import type { TranslationEntry } from "@/i18n";

const deLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "Bedingungen",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "Nutzungsbedingungen für Agent-Modelle",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "Gültig ab {date}",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "Geltungsbereich",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "Diese Bedingungen gelten, wenn die Agent-Sitzungen eines Workspace das von bex bereitgestellte Modell statt des eigenen Modellanbieter-Schlüssels des Workspace verwenden.",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "Deine Daten",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "Um Antworten zu generieren, sendet bex die Prompts, den Code und die Tool-Ausgaben der Sitzung über OpenRouter an Modellanbieter von Drittanbietern.",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "Anfragen werden nur an Anbieter weitergeleitet, die Prompts nicht über die Verarbeitung hinaus speichern und nicht damit trainieren: bex nutzt das Provider-Routing von OpenRouter mit verweigerter Datenerfassung.",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Muse-Code-Sitzungen sind die Ausnahme. bex sendet sie direkt an die Modell-API von Meta in deren Contributor-Tier, unter der Meta die gesendeten Prompts, den Code, Dateien und Kontext sowie die zurückgegebenen Antworten zum Training seiner Modelle verwenden darf. Muse Code bleibt in einem Workspace aus, bis ein Admin es einschaltet und dies bestätigt.",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "Wenn der Workspace stattdessen einen eigenen Anbieter-Schlüssel verwendet, gehen Anfragen im Rahmen deiner Vereinbarung mit deinem Anbieter an diesen, und bex berechnet keine Modell-Tokens.",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "Preis",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "Jede Anfrage wird zu den tatsächlichen Kosten des Modellanbieters plus 30 % abgerechnet. Die Nutzung wird pro Anfrage gemessen und unter Abrechnung → Agent-Modell angezeigt.",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "Ausgabenlimits",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "Jeder Workspace hat ein monatliches Ausgabenlimit, das sein Tarif festlegt: Hobby 20 $, Pro 200 $, Scale 2.000 $. Workspace-Admins können es bis auf das Zehnfache des Tarif-Limits anpassen.",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "Ein neuer Workspace kann bis zu seiner ersten erfolgreichen Zahlung zwei gleichzeitige Sitzungen mit dem von bex bereitgestellten Modell ausführen.",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "Ist das Limit erreicht, werden neue Modellanfragen abgelehnt, bis der nächste Monat beginnt oder ein Admin das Limit erhöht.",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "Abbuchungen im Laufe des Monats",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "Du ermächtigst bex, die Zahlungsmethode des Workspace zusätzlich zur Monatsrechnung zu belasten, sobald die noch nicht abgerechnete Agent-Modell-Nutzung den Abbuchungsschwellenwert des Workspace erreicht. Der Workspace arbeitet während der Abbuchung weiter. Der Schwellenwert beginnt bei 10 $ und steigt mit der Zahlungshistorie des Workspace.",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "Schlägt eine Abbuchung fehl oder erfordert sie deine Authentifizierung, wird die Agent-Modell-Nutzung pausiert, bis der Betrag bezahlt ist.",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "Löschen eines Workspace oder Accounts",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "Bevor ein Workspace oder Account gelöscht wird, bucht bex die noch nicht abgerechnete Nutzung ab. Schlägt diese Abbuchung fehl, wartet das Löschen, bis der Betrag bezahlt ist.",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "Du ermächtigst bex, die vor dem Löschen angefallene Nutzung abzurechnen, auch nach dem Löschen.",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "Zahlungsmethoden",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "Prepaid-Karten werden nicht akzeptiert. Eine Karte, die mit einem offenen bex-Betrag verknüpft ist, kann abgelehnt werden.",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "Nutzungsnachweise",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex führt 13 Monate lang einen Nachweis über jede Anfrage (Zeit, Modell, Tokens und Preis). Diese Nachweise können als Beweis herangezogen werden, wenn eine Abbuchung angefochten wird.",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "Änderungen dieser Bedingungen",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "bex kann diese Bedingungen aktualisieren. Das Datum oben auf dieser Seite zeigt die aktuelle Version.",
    description: "Terms: changes",
  },
};

export default deLegal;
