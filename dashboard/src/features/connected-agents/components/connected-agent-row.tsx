import { TableRow, TableCell } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { RevokeIconButton } from "@/common/components/revoke-icon-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import { SCOPE_DESCRIPTION_KEYS, SCOPE_SENSITIVE, SCOPE_WRITE } from "@/common/lib/oauth-scopes";
import type { ConnectedAgentView } from "@/features/connected-agents/types";

export interface ConnectedAgentRowProps {
  agent: ConnectedAgentView;
  onRevoke: (clientId: string, clientName: string) => Promise<boolean>;
  /** True while this row's revoke is in flight — disables its own control. */
  revoking: boolean;
}

/** One Connected Agents row: client, granted scopes, grant date, and revoke behind a confirmation. */
export function ConnectedAgentRow({
  agent,
  onRevoke,
  revoking,
}: ConnectedAgentRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="max-w-[12rem] truncate font-medium">
        {agent.clientUri ? (
          <a
            href={agent.clientUri}
            target="_blank"
            rel="noreferrer noopener"
            className="hover:underline"
          >
            {agent.clientName}
          </a>
        ) : (
          agent.clientName
        )}
      </TableCell>
      <TableCell className="max-w-[16rem]">
        <div className="flex flex-wrap gap-1">
          {agent.scopes.map((scope) => (
            <Badge
              key={scope}
              variant={
                scope === SCOPE_WRITE || scope === SCOPE_SENSITIVE
                  ? "destructive"
                  : "secondary"
              }
              className="font-mono text-xs"
              title={
                SCOPE_DESCRIPTION_KEYS[scope]
                  ? t(SCOPE_DESCRIPTION_KEYS[scope])
                  : scope
              }
            >
              {scope}
            </Badge>
          ))}
        </div>
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {formatRelativeAge(agent.grantedAt)}
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <RevokeIconButton
          label={t("connectedAgents.revoke")}
          confirmTitle={t("connectedAgents.revokeConfirmTitle", {
            name: agent.clientName,
          })}
          confirmBody={t("connectedAgents.revokeConfirmBody")}
          cancelLabel={t("connectedAgents.revokeCancel")}
          confirmLabel={t("connectedAgents.revoke")}
          onConfirm={() => void onRevoke(agent.clientId, agent.clientName)}
          pending={revoking}
        />
      </TableCell>
    </TableRow>
  );
}
