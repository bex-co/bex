import type { TranslationEntry } from "@/i18n";

const deUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Abrechnung",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message:
      "Zahlung, Rechnungen und der bisherige Monatsverbrauch des Workspace",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "Abrechnungsbereiche",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "Services",
    description: "Service count label in the resource-cap card",
  },
  "usage.resourceCapsPostgres": {
    message: "Postgres",
    description: "Postgres count label in the resource-cap card",
  },
  "usage.resourceCapsKeyValues": {
    message: "Key Value",
    description: "Key Value count label in the resource-cap card",
  },
  "usage.resourceCapsFree": {
    message: "Free: {used} von {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "Monatlich enthaltene Nutzung",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "Free Web Services teilen sich pro Workspace ein monatliches Kontingent an Instanzstunden. Zeit im heruntergefahrenen Zustand zählt nicht.",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message:
      "{used} von {included} Free-Instanzstunden in diesem Monat verbraucht",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "Du bist nah am Kontingent. Wenn es aufgebraucht ist, werden deine Free Web Services bis zum Beginn des nächsten Monats pausiert.",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "Das Kontingent ist aufgebraucht. Free Web Services sind bis zum Beginn des nächsten Monats pausiert; stelle einen Service auf einen kostenpflichtigen Tarif um, um ihn jetzt auszuführen.",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "Pipeline-Minuten",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "Builds und Pre-Deploy-Befehle verbrauchen Pipeline-Minuten. Dein Tarif enthält jeden Monat einige; darüber hinausgehende Minuten werden pro Minute abgerechnet.",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message:
      "{used} von {included} enthaltenen Build-Minuten in diesem Monat verbraucht",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "Du bist nah an deinen enthaltenen Build-Minuten. Ohne Zahlungsmethode werden neue Builds und Pre-Deploy-Befehle gestoppt, wenn sie aufgebraucht sind, bis zum Beginn des nächsten Monats.",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "Die enthaltenen Build-Minuten sind aufgebraucht. Weitere Minuten werden pro Minute abgerechnet; ohne Zahlungsmethode werden neue Builds und Pre-Deploy-Befehle bis zum Beginn des nächsten Monats gestoppt.",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "Ausgehende Bandbreite",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "Traffic, den deine Services ins Internet senden, verbraucht Bandbreite. Dein Tarif enthält jeden Monat einen Teil; darüber hinausgehende Bandbreite wird pro GB abgerechnet.",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message:
      "{used} von {included} GB enthaltener Bandbreite in diesem Monat verbraucht",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "Du bist nah an deiner enthaltenen Bandbreite. Ohne Zahlungsmethode werden die Services deines Workspace heruntergefahren, wenn sie aufgebraucht ist, bis zum Beginn des nächsten Monats.",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "Die enthaltene Bandbreite ist aufgebraucht. Weitere Bandbreite wird pro GB abgerechnet; ohne Zahlungsmethode werden die Services deines Workspace bis zum Beginn des nächsten Monats heruntergefahren.",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.customDomainsTitle": {
    message: "Benutzerdefinierte Domains",
    description:
      "Title of the included custom domains card on the billing page (w7/m429)",
  },
  "usage.customDomainsDescription": {
    message:
      "Dein Plan enthält eine Anzahl verifizierter benutzerdefinierter Domains. Jede weitere Domain wird pro Monat berechnet, für die Zeit, in der sie verifiziert ist. Das Hinzufügen einer Domain wird nie wegen der Anzahl abgelehnt.",
    description:
      "Explains the plan's included custom domains and how domains beyond them are billed (w7/m429)",
  },
  "usage.customDomainsUsed": {
    message:
      "{used} von {included} enthaltenen benutzerdefinierten Domains in Verwendung",
    description:
      "Verified custom domains now against the count the plan includes (w7/m429)",
  },
  "usage.customDomainsNear": {
    message:
      "Du näherst dich den in deinem Plan enthaltenen benutzerdefinierten Domains. Jede weitere Domain wird pro Monat berechnet.",
    description:
      "Note once the verified custom domains reach 80 percent of the included count (w7/m429)",
  },
  "usage.customDomainsSpent": {
    message:
      "Die in deinem Plan enthaltenen benutzerdefinierten Domains sind in Verwendung. Jede weitere verifizierte Domain wird pro Monat berechnet.",
    description: "Notice once every included custom domain is in use (w7/m429)",
  },
  "usage.categoryCustomDomains": {
    message: "Benutzerdefinierte Domains",
    description:
      "Charge-tree category for a workspace's custom domains beyond its plan (w7/m429)",
  },
  "usage.chargeCronMinimum": {
    message: "Cron-Job-Mindestbetrag",
    description:
      "Charge line under a cron job: what its runs in a month cost short of the cron job minimum, charged once the month ends (w9/m448)",
  },
  "usage.chargeCronMinimumRate": {
    message: "Mindestens ${minimum}/Monat",
    description:
      "Rate column value for the cron job minimum line; minimum is the USD amount without the currency symbol (w9/m448)",
  },
  "usage.chargeCustomDomains": {
    message: "Benutzerdefinierte Domains über den Plan hinaus",
    description:
      "Charge line for custom domains beyond the plan's included count, billed per domain-month (w7/m429)",
  },
  "usage.chargeIncluded": {
    message: "{included} enthalten",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "Build-Pipeline",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "Builds und Pre-Deploy-Befehle verbrauchen Pipeline-Minuten. Minuten über die in deinem Tarif enthaltenen hinaus werden pro Minute abgerechnet, bis zum monatlichen Ausgabenlimit, das du festlegst. Wenn das Limit erreicht ist, werden Builds bis zum nächsten Monat gestoppt.",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "Ausgabenlimit: {limit} pro Monat",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "Kein Ausgabenlimit",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "Ausgabenlimit festlegen",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "Bearbeiten",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "Limit entfernen",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "Ausgabenlimit festlegen",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "Der Höchstbetrag, den dieser Workspace pro Monat für Build-Minuten über die im Tarif enthaltenen hinaus ausgibt. Wenn er erreicht ist, werden neue Builds und Pre-Deploy-Befehle bis zum Beginn des nächsten Monats gestoppt.",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "Monatliches Limit (USD)",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "Speichern",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "Ausgabenlimit auf {limit} pro Monat festgelegt",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "Ausgabenlimit entfernt",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "Das Ausgabenlimit konnte nicht aktualisiert werden",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message:
      "Das Ausgabenlimit konnte nicht geladen werden. Lade die Seite neu und versuche es erneut.",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message:
      "Nur Abrechnungsverantwortliche können das Ausgabenlimit sehen und ändern.",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "Nahe am Limit",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "Limit erreicht",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "{count} werden noch gelöscht",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "Zählt weiter zu deinem Limit, bis das Löschen abgeschlossen ist",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.billingDetailsDeniedTitle": {
    message: "Abrechnungsdetails sind für deine Rolle nicht verfügbar",
    description:
      "Billing page heading for a Contributor or a Viewer, whose role may not view billing details (Render's View billing details row, w9/m620)",
  },
  "usage.errorTitle": {
    message: "Nutzung konnte nicht geladen werden",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "Monat auswählen",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "Aktueller Monat",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "Verbleibendes Guthaben",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message:
      "Aktionsguthaben, das auf Rechnungen angerechnet wird, bevor deine Zahlungsmethode belastet wird",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "{amount} davon verfallen am {date}",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "Auch mit Guthaben ist weiterhin eine Zahlungsmethode erforderlich: Das Guthaben deckt Rechnungen zuerst, und deine Karte zahlt einen etwaigen Rest.",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "Zeitraum",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "Status",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "Betrag",
    description: "Table column header for an invoice's amount",
  },
  "usage.colInvoiceActions": {
    message: "Aktionen",
    description: "Screen-reader header of the invoice table's row-menu column",
  },
  "usage.billingSetupTitle": {
    message: "Abrechnungseinrichtung",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "Zahlungseinzug und Steuerbereitschaft für diesen Workspace",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe-Testmodus",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "Bereit",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "Aktion erforderlich",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "Kunde",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "Nutzungsbasiertes Abonnement",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "Zahlungsmethode",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "Automatische Steuer",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "Einzugsstatus",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "Zahlung fehlgeschlagen ({reason}). Der Workspace bleibt während der Kulanzfrist verfügbar; eine umkehrbare Sperrung ist nach {deadline} geplant.",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "Die Abrechnungsdurchsetzung ist aktiv ({reason}). Compute ist pausiert, aber Datenbanken, Key-Value-Daten, Secrets und Abrechnungsnachweise wurden nicht gelöscht.",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "Zahlung eingegangen. bex stellt nur die Ressourcen wieder her, die durch die Abrechnungsdurchsetzung geändert wurden.",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message:
      "Dieser Workspace ist von einem Operator vom Stripe-Einzug ausgenommen.",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message:
      "Dieser Workspace wird bewertet, ist aber von einem Operator vollständig kostenfrei gestellt.",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message:
      "Der Abrechnungsstatus ist {reason}. Nutze das Abrechnungsportal oder wende dich an den Support.",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "keine Frist gemeldet",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "Aus",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "Zahlungsmethode hinzufügen",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "Zahlungsmethode aktualisieren",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "Test-Zahlungsmethode hinzufügen",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "Test-Zahlungsmethode aktualisieren",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "Abrechnungsportal öffnen",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "Zahlungsdaten werden nur auf von Stripe gehosteten Seiten eingegeben. bex sendet nie einen Stripe-Server-Schlüssel an deinen Browser.",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message:
      "Die Abrechnungsdetails sind gerade nicht verfügbar. Versuch es später erneut.",
    description:
      "Billing readiness could not be read (a Stripe outage, the integration off). Every role that reaches the card may view billing details since w9/m620, so this is never a role refusal",
  },
  "usage.billingCheckoutError": {
    message:
      "Stripe Checkout konnte nicht geöffnet werden. Versuche es erneut.",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message:
      "Das Stripe-Abrechnungsportal konnte nicht geöffnet werden. Versuche es erneut.",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "Füge eine Zahlungsmethode hinzu, um fortzufahren",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "Dieser kostenpflichtige Tarif benötigt eine Zahlungsmethode. Schließe Stripe Checkout im neuen Tab ab; diese Aktion wird automatisch fortgesetzt, sobald alles bereit ist.",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "Zahlungsmethode gefunden. Deine Aktion wird erneut versucht…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "Abbrechen",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "Tarif",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "Tarif ändern",
    description:
      "Link from the billing page's plan card to Render's plan page (w3/m708)",
  },
  "usage.paymentMethodTitle": {
    message: "Zahlungsmethode",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} mit Endziffern {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "Zahlungsmethode hinterlegt",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "Keine Zahlungsmethode",
    description: "Shown when the workspace has no payment method",
  },
  "usage.billingEmailTitle": {
    message: "Rechnungs-E-Mail",
    description:
      "Billing email card title and section navigation entry on the billing page (w6/m627)",
  },
  "usage.billingEmailDescription": {
    message:
      "Abrechnungshinweise für diesen Workspace gehen an diese Adresse, und seine Rechnungen sind an sie adressiert.",
    description: "Billing email card description: what the address is used for",
  },
  "usage.billingEmailHobbyNote": {
    message:
      "Hobby-Workspaces verwenden die Konto-E-Mail des Inhabers. Eine eigene Rechnungs-E-Mail gibt es ab Pro.",
    description:
      "Note under a Hobby workspace's billing email, which is read-only and is the owner's account email",
  },
  "usage.billingEmailNone": {
    message: "Keine Rechnungs-E-Mail",
    description:
      "Shown when neither a billing email nor the owner's account email is known",
  },
  "usage.billingEmailUnreadable": {
    message: "Die Rechnungs-E-Mail konnte nicht geladen werden.",
    description: "Shown when the billing email could not be read",
  },
  "usage.billingEmailEdit": {
    message: "Bearbeiten",
    description: "Button that opens the billing email dialog (Pro and higher)",
  },
  "usage.billingEmailDialogTitle": {
    message: "Rechnungs-E-Mail bearbeiten",
    description: "Title of the dialog that sets the billing email",
  },
  "usage.billingEmailDialogDescription": {
    message:
      "Abrechnungshinweise gehen außerdem an die Admins des Workspace unter ihren eigenen Adressen.",
    description: "Description in the billing email dialog",
  },
  "usage.billingEmailLabel": {
    message: "E-Mail-Adresse",
    description: "Label of the billing email input",
  },
  "usage.billingEmailSave": {
    message: "Speichern",
    description: "Save button of the billing email dialog",
  },
  "usage.billingEmailSaved": {
    message: "Rechnungs-E-Mail auf {email} gesetzt",
    description:
      "Toast after the billing email was saved; {email} is the new address",
  },
  "usage.billingEmailError": {
    message: "Die Rechnungs-E-Mail konnte nicht aktualisiert werden",
    description: "Fallback toast when the billing email could not be saved",
  },
  "usage.billingInfoTitle": {
    message: "Rechnungsangaben",
    description:
      "Billing info card title and section navigation entry on the billing page (w6/m628)",
  },
  "usage.billingInfoDescription": {
    message:
      "Firmenname, Adresse und USt-IdNr. oder Steuernummer auf jeder Rechnung.",
    description: "Billing info card description: what the billing info is for",
  },
  "usage.billingInfoBillTo": {
    message: "Rechnung an",
    description:
      "Label of the name invoices bill: the company name when set, else the workspace's name",
  },
  "usage.billingInfoCompanyName": {
    message: "Firmenname",
    description:
      "Label of the company name, on the billing info card and in its dialog",
  },
  "usage.billingInfoAddress": {
    message: "Adresse",
    description: "Label of the billing address on the billing info card",
  },
  "usage.billingInfoTaxId": {
    message: "Steuernummer",
    description:
      "Label of the VAT or tax ID on the billing info card, and of its value in the dialog when the country has several types",
  },
  "usage.billingInfoNotSet": {
    message: "Nicht festgelegt",
    description: "Shown for a part of the billing info that is not set",
  },
  "usage.billingInfoUnreadable": {
    message: "Die Rechnungsangaben konnten nicht geladen werden.",
    description: "Shown when the billing info could not be read",
  },
  "usage.billingInfoCustomerNotReady": {
    message:
      "Füge zuerst eine Zahlungsmethode hinzu; die Rechnungsangaben werden zusammen mit ihr gespeichert.",
    description:
      "Note on the billing info card, and error toast, while the workspace has no billing account (no payment method was ever added)",
  },
  "usage.billingInfoAdd": {
    message: "Rechnungsangaben hinzufügen",
    description:
      "Button that opens the billing info dialog when no billing info is set",
  },
  "usage.billingInfoEdit": {
    message: "Bearbeiten",
    description:
      "Button that opens the billing info dialog when billing info is set",
  },
  "usage.billingInfoDialogTitle": {
    message: "Rechnungsangaben",
    description: "Title of the dialog that sets the billing info",
  },
  "usage.billingInfoDialogDescription": {
    message:
      "Zusätzliche Rechnungsangaben wie Firmenname, Adresse oder USt-IdNr., die ab jetzt auf allen Rechnungen stehen.",
    description: "Description in the billing info dialog (Render's wording)",
  },
  "usage.billingInfoCompanyNameHint": {
    message: "Leer lassen, um auf den Namen des Workspace abzurechnen.",
    description:
      "Hint under the company name field: an empty company name bills the workspace's name",
  },
  "usage.billingInfoCountry": {
    message: "Land",
    description: "Label of the billing address country picker",
  },
  "usage.billingInfoCountryPlaceholder": {
    message: "Land auswählen",
    description: "Placeholder of the country picker when no country is chosen",
  },
  "usage.billingInfoNoCountry": {
    message: "Kein Land",
    description:
      "Country picker choice that clears the country (and so the address)",
  },
  "usage.billingInfoCountryRequired": {
    message: "Wähle ein Land aus",
    description:
      "Error under the country picker when an address line was typed without a country",
  },
  "usage.billingInfoLine1": {
    message: "Adresszeile 1",
    description: "Label of the first street address line",
  },
  "usage.billingInfoLine2": {
    message: "Adresszeile 2",
    description: "Label of the second street address line",
  },
  "usage.billingInfoCity": {
    message: "Stadt",
    description: "Label of the city field",
  },
  "usage.billingInfoState": {
    message: "Bundesland / Provinz / Region",
    description: "Label of the state, province or region field",
  },
  "usage.billingInfoPostalCode": {
    message: "Postleitzahl",
    description: "Label of the postal code field",
  },
  "usage.billingInfoTaxIdType": {
    message: "Art der Steuernummer",
    description:
      "Label of the tax ID type picker, shown when the country has several types",
  },
  "usage.billingInfoSave": {
    message: "Speichern",
    description: "Save button of the billing info dialog",
  },
  "usage.billingInfoSaved": {
    message: "Rechnungsangaben gespeichert. Neue Rechnungen gehen an {name}.",
    description:
      "Toast after the billing info was saved; {name} is the name new invoices bill",
  },
  "usage.billingInfoError": {
    message: "Die Rechnungsangaben konnten nicht gespeichert werden",
    description: "Fallback toast when the billing info could not be saved",
  },
  "usage.includedUsageTitle": {
    message: "Enthaltene Nutzung",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message:
      "Was der Tarif dieses Workspace enthält und wie viel davon genutzt wird.",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "Kosten",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "In diesem Zeitraum bisher angefallen, bewertet nach der bex-Preisliste. Eine Schätzung, keine Rechnung.",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message: "In diesem Zeitraum bisher angefallen, wie von Stripe bewertet.",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "Fälliger Betrag nach Guthaben",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "Keine Nutzung in diesem Zeitraum.",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "Teilweise Daten",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "Diese Schätzung zählt die Nutzung möglicherweise zu niedrig.",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "Gemessen bis {through}.",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "Beeinträchtigte Messquellen: {sources}.",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "Alle ausklappen",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "Alle einklappen",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "Gesamt seit Monatsbeginn",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "Gesamt für den Zeitraum",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "Gesamt in diesem Abrechnungszeitraum ({start} – {end})",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "Gesamt in diesem Abrechnungszeitraum",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "Aufschlüsselung oben: {month} bisher",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "Prognostizierte Gesamtsumme für {month}",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "Prognose für diesen Abrechnungszeitraum",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "Enthalten",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "(gelöscht)",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "Services",
    description: "Charge-tree category for App services",
  },
  "usage.categoryPostgres": {
    message: "Postgres",
    description: "Charge-tree category for managed Postgres instances",
  },
  "usage.categoryKeyValue": {
    message: "Key Value",
    description: "Charge-tree category for managed Key Value instances",
  },
  "usage.categoryObjectStorage": {
    message: "Objektspeicher",
    description:
      "Charge-tree category for a workspace's object storage (ADR112)",
  },
  "usage.categoryAgentSessions": {
    message: "Agent-Sitzungen",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "Disk",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "Modellnutzung",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "Anbieterkosten + 30 %",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "Input-Tokens",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "Output-Tokens",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "Cache-Lese-Tokens",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "Cache-Schreib-Tokens",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "Sandboxes",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "Compute",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "Bandbreite",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "Build-Minuten",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "Speicher",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "Sandbox-Compute",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "Sandbox-Arbeitsspeicher",
    description:
      "Charge line for the memory a running sandbox was allocated, billed per GiB-hour since billing switched to use",
  },
  "usage.chargeSandboxMemoryUnbilled": {
    message: "Sandbox-Arbeitsspeicher (in Compute enthalten)",
    description:
      "Charge line for the allocated memory a sandbox ran with before billing switched to use; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxCpu": {
    message: "Sandbox-CPU",
    description:
      "Charge line for the CPU a running sandbox used, billed per CPU-hour used",
  },
  "usage.chargeSandboxCpuUnbilled": {
    message: "Sandbox-CPU (in Compute enthalten)",
    description:
      "Charge line for the CPU a sandbox used before billing switched to use; recorded beside sandbox compute, which already charged for it, so it is shown and not billed separately",
  },
  "usage.sandboxUsageBilledFromNote": {
    message:
      "Seit {date} UTC wird eine laufende Sandbox nach genutzter CPU und zugewiesenem Arbeitsspeicher abgerechnet. Frühere Sandbox-Nutzung wurde als Sandbox-Compute abgerechnet.",
    description:
      "Note under the charge tree of a period with sandbox usage; date is the instant billing switched, formatted in UTC",
  },
  "usage.agentSandboxUsageBilledFromNote": {
    message:
      "Die Sandbox einer Agent-Sitzung wird ab {date} UTC so abgerechnet; davor wird ihre Nutzung als Sandbox-Compute abgerechnet.",
    description:
      "Second sentence of the note under the charge tree of a period with sandbox usage; date is the instant an agent session's sandbox switched from sandbox compute to used CPU and allocated memory, formatted in UTC (it may be in the future)",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "Sandbox-Snapshot-Speicher",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.chargeObjectStorage": {
    message: "Speicher",
    description:
      "Charge line for the data a workspace keeps in object storage, billed per GB-month (shown at no charge until a price is set, ADR112 D8)",
  },
  "usage.chargeObjectStorageIngress": {
    message: "Eingehende Bandbreite",
    description:
      "Charge line for the bytes put into a workspace's object storage (shown at no charge, ADR112 D8)",
  },
  "usage.chargeObjectStorageEgress": {
    message: "Ausgehende Bandbreite",
    description:
      "Charge line for the bytes read out of a workspace's object storage, public reads included (shown at no charge, ADR112 D8)",
  },
  "usage.creditsTotalLabel": {
    message: "Gesamtguthaben",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "Rechnungsverlauf",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "Deine abgeschlossenen Rechnungen.",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "Agent-Modellkosten",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceActionsMenu": {
    message: "Aktionen für die Rechnung {period}",
    description:
      "Accessible label of an invoice row's three-dot menu; {period} is the invoice's month",
  },
  "usage.invoicePayNow": {
    message: "Jetzt bezahlen",
    description:
      "Invoice row menu item that opens Stripe's hosted page to pay an unpaid invoice (Render's Pay Now)",
  },
  "usage.invoiceDownloadPdf": {
    message: "PDF herunterladen",
    description: "Invoice row menu item that downloads the invoice as a PDF",
  },
  "usage.invoiceStatusOpen": {
    message: "Unbezahlt",
    description:
      "Status of an invoice that is finalized and not yet paid (Stripe: open)",
  },
  "usage.invoiceStatusPaid": {
    message: "Bezahlt",
    description: "Status of a paid invoice (Stripe: paid)",
  },
  "usage.invoiceStatusVoid": {
    message: "Storniert",
    description:
      "Status of a voided invoice, which is owed no longer (Stripe: void)",
  },
  "usage.invoiceStatusUncollectible": {
    message: "Uneinbringlich",
    description:
      "Status of an invoice written off as uncollectible, which can still be paid (Stripe: uncollectible)",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "Im Laufe eines Monats angefallene Agent-Modellkosten werden von der Rechnung dieses Monats abgezogen.",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "Angerechnetes Guthaben −{applied} → fälliger Betrag {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message:
      "In diesem Zeitraum bisher angefallen, bewertet nach der bex-Preisliste.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message:
      "Prepaid-Karten werden nicht akzeptiert; füge eine Kredit- oder Debitkarte hinzu.",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message:
      "Diese Karte ist mit einem offenen Saldo verknüpft; füge eine andere Karte hinzu.",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "Agent-Modell",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "Abgerechnet zu den Kosten des Modellanbieters + 30 %.",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "Seit Monatsbeginn",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "Gesamt für den Monat",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_one": {
    message: "Aufschlüsselung nach Modell ({count} Modell)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_other": {
    message: "Aufschlüsselung nach Modell ({count} Modelle)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "Keine Agent-Modellnutzung in diesem Zeitraum.",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
  "usage.pipelineMinutesExceeded": {
    message:
      "Du hast die maximale Anzahl an {minutes} aufgebraucht, die dein Ausgabenlimit festlegt. {learnMore}.",
    description:
      "Why Manual Build and Manual Deploy are disabled while the workspace's pipeline minutes are spent; {minutes} is a link to Workspace Settings → Build Pipeline named by usage.pipelineMinutesLink, {learnMore} a link to the docs named by usage.pipelineMinutesLearnMore, which starts its own line (Render's text, w3/108)",
  },
  "usage.pipelineMinutesLink": {
    message: "Pipeline-Minuten",
    description:
      "Link text inside usage.pipelineMinutesExceeded, to Workspace Settings → Build Pipeline (Render's text, w3/108)",
  },
  "usage.pipelineMinutesLearnMore": {
    message: "Mehr erfahren",
    description:
      "Link text inside usage.pipelineMinutesExceeded, to the pipeline minutes docs (Render's text, w3/108)",
  },
};

export default deUsage;
