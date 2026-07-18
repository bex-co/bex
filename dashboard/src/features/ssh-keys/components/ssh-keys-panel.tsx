import { useState } from "react";
import { KeyRound, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useSSHKeys } from "@/features/ssh-keys/hooks/use-ssh-keys";

const publicKeyPattern =
  /^(ssh-(?:ed25519|rsa)|ecdsa-sha2-nistp(?:256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com)[ \t]+[A-Za-z0-9+/]+={0,3}(?:[ \t]+[^\r\n]+)?$/;

export function SSHKeysPanel() {
  const { t } = useTranslations();
  const state = useSSHKeys();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [publicKey, setPublicKey] = useState("");
  const valid =
    name.trim().length > 0 && publicKeyPattern.test(publicKey.trim());

  async function create() {
    if (!valid) return;
    if (await state.create(name.trim(), publicKey.trim())) {
      setName("");
      setPublicKey("");
      setOpen(false);
    }
  }

  return (
    <Card id="ssh-public-keys" className="scroll-mt-4">
      <CardHeader>
        <CardTitle>{t("sshKeys.title")}</CardTitle>
        <CardDescription>{t("sshKeys.description")}</CardDescription>
        <CardAction>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button variant="outline" size="sm">
                <Plus />
                {t("sshKeys.add")}
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("sshKeys.addTitle")}</DialogTitle>
                <DialogDescription>
                  {t("sshKeys.addDescription")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="ssh-key-name">{t("sshKeys.name")}</Label>
                  <Input
                    id="ssh-key-name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    autoComplete="off"
                    maxLength={120}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="ssh-public-key">
                    {t("sshKeys.publicKey")}
                  </Label>
                  <textarea
                    id="ssh-public-key"
                    className="border-input min-h-28 w-full rounded-md border bg-transparent px-3 py-2 font-mono text-xs"
                    value={publicKey}
                    onChange={(event) => setPublicKey(event.target.value)}
                    autoComplete="off"
                    autoCapitalize="none"
                    autoCorrect="off"
                    maxLength={16384}
                    spellCheck={false}
                  />
                  {publicKey && !publicKeyPattern.test(publicKey.trim()) ? (
                    <p className="text-destructive text-sm">
                      {t("sshKeys.invalid")}
                    </p>
                  ) : null}
                </div>
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">{t("sshKeys.cancel")}</Button>
                </DialogClose>
                <Button
                  onClick={() => void create()}
                  disabled={!valid || state.busy === "create"}
                >
                  {state.busy === "create" ? (
                    <Loader2 className="animate-spin" />
                  ) : null}
                  {t("sshKeys.save")}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardAction>
      </CardHeader>
      <CardContent>
        {state.error ? (
          <PanelCenteredState
            icon={<KeyRound />}
            title={t("sshKeys.errorTitle")}
            body={t("sshKeys.errorBody")}
          />
        ) : state.loading && state.keys.length === 0 ? (
          <PanelTableSkeleton />
        ) : state.keys.length === 0 ? (
          <PanelCenteredState
            icon={<KeyRound />}
            title={t("sshKeys.emptyTitle")}
            body={t("sshKeys.emptyBody")}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("sshKeys.name")}</TableHead>
                <TableHead>{t("sshKeys.fingerprint")}</TableHead>
                <TableHead>{t("sshKeys.created")}</TableHead>
                <TableHead className="sr-only">
                  {t("sshKeys.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {state.keys.map((key) => (
                <TableRow key={key.id}>
                  <TableCell>{key.name}</TableCell>
                  <TableCell
                    className="max-w-72 truncate font-mono text-xs"
                    title={key.fingerprint}
                  >
                    {key.fingerprint}
                  </TableCell>
                  <TableCell className="text-muted-foreground whitespace-nowrap">
                    {formatRelativeAge(key.createdAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          size="icon"
                          variant="ghost"
                          aria-label={t("sshKeys.delete")}
                          disabled={state.busy === key.id}
                        >
                          {state.busy === key.id ? (
                            <Loader2 className="animate-spin" />
                          ) : (
                            <Trash2 className="text-destructive" />
                          )}
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>
                            {t("sshKeys.deleteTitle", { name: key.name })}
                          </AlertDialogTitle>
                          <AlertDialogDescription>
                            {t("sshKeys.deleteBody")}
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>
                            {t("sshKeys.cancel")}
                          </AlertDialogCancel>
                          <AlertDialogAction
                            onClick={() => void state.remove(key.id, key.name)}
                          >
                            {t("sshKeys.delete")}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
