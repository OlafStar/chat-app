'use client'

import * as React from "react"
import { format } from "date-fns"
import {
  CheckIcon,
  CopyIcon,
  KeyRoundIcon,
  Loader2Icon,
  RefreshCwIcon,
  ShieldCheckIcon,
  Trash2Icon,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "~/components/ui/alert"
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
} from "~/components/ui/alert-dialog"
import { Badge } from "~/components/ui/badge"
import { Button } from "~/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "~/components/ui/empty"
import { Separator } from "~/components/ui/separator"
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table"
import { getAPIErrorMessage } from "~/lib/errors"
import {
  useCreateTenantApiKeyMutation,
  useDeleteTenantApiKeyMutation,
  useTenantApiKeys,
} from "~/hooks/use-api-keys"

const widgetScriptUrl =
  process.env.NEXT_PUBLIC_WIDGET_SCRIPT_URL ??
  "http://localhost:8090/pingy-chat-widget.js"

const defaultKeyPlaceholder = "<your-api-key>"

export default function DeveloperPage() {
  const [copiedKeyId, setCopiedKeyId] = React.useState<string | null>(null)
  const [snippetCopied, setSnippetCopied] = React.useState(false)
  const [deletingKeyId, setDeletingKeyId] = React.useState<string | null>(null)
  const [clipboardError, setClipboardError] = React.useState<string | null>(null)

  const {
    data: apiKeys,
    isLoading,
    isError,
    error,
    refetch,
  } = useTenantApiKeys()
  const createKeyMutation = useCreateTenantApiKeyMutation()
  const deleteKeyMutation = useDeleteTenantApiKeyMutation()

  const activeKey = apiKeys?.[0]?.apiKey
  const embedSnippet = React.useMemo(() => {
    const tenantKey = activeKey ?? defaultKeyPlaceholder
    return [
      `<script src="${widgetScriptUrl}" async></script>`,
      `<script>`,
      `  window.PingyChatWidget = window.PingyChatWidget || {};`,
      `  PingyChatWidget.init({`,
      `    tenantKey: "${tenantKey}",`,
      `  });`,
      `</script>`,
    ].join("\n")
  }, [activeKey])

  const handleCopy = async (
    value: string,
    onSuccess: () => void,
    onError: (message: string) => void
  ) => {
    try {
      if (!navigator?.clipboard) {
        throw new Error("Clipboard access is not available in this browser")
      }
      await navigator.clipboard.writeText(value)
      onSuccess()
    } catch (err) {
      onError(getAPIErrorMessage(err, "Unable to copy to clipboard"))
    }
  }

  const handleCreateKey = () => {
    createKeyMutation.mutate()
  }

  const handleDeleteKey = (keyId: string) => {
    setDeletingKeyId(keyId)
    deleteKeyMutation.mutate(keyId, {
      onSettled: () => setDeletingKeyId(null),
    })
  }

  const handleCopySnippet = () => {
    handleCopy(
      embedSnippet,
      () => {
        setSnippetCopied(true)
        setClipboardError(null)
        setTimeout(() => setSnippetCopied(false), 2000)
      },
      (message) => setClipboardError(message)
    )
  }

  const handleCopyKeyValue = (keyId: string, apiKey: string) => {
    handleCopy(
      apiKey,
      () => {
        setCopiedKeyId(keyId)
        setClipboardError(null)
        setTimeout(() => setCopiedKeyId(null), 2000)
      },
      (message) => setClipboardError(message)
    )
  }

  const listError = isError ? getAPIErrorMessage(error) : null
  const mutationError =
    createKeyMutation.error || deleteKeyMutation.error
      ? getAPIErrorMessage(createKeyMutation.error ?? deleteKeyMutation.error)
      : null

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <KeyRoundIcon className="text-primary size-5" />
          <h1 className="text-2xl font-semibold">Developer & API Access</h1>
        </div>
        <p className="text-muted-foreground max-w-2xl text-sm">
          Manage tenant API tokens and drop-in chat embed code. Keys in this
          panel are scoped to your tenant—rotate them regularly and remove any
          tokens that are no longer in use.
        </p>
      </div>

      {listError && (
        <Alert variant="destructive">
          <AlertTitle>Couldn&apos;t load API tokens</AlertTitle>
          <AlertDescription className="flex items-center gap-3">
            <span>{listError}</span>
            <Button size="sm" variant="secondary" onClick={() => refetch()}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {mutationError && !listError && (
        <Alert variant="destructive">
          <AlertTitle>Action failed</AlertTitle>
          <AlertDescription>{mutationError}</AlertDescription>
        </Alert>
      )}

      {clipboardError && (
        <Alert variant="destructive">
          <AlertTitle>Copy failed</AlertTitle>
          <AlertDescription>{clipboardError}</AlertDescription>
        </Alert>
      )}

      <Card className="border-primary/20 bg-gradient-to-br from-primary/5 via-background to-background">
        <CardHeader className="gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1">
            <CardTitle>Embed the chat widget</CardTitle>
            <CardDescription>
              Paste this snippet before the closing <code>&lt;/body&gt;</code>{" "}
              tag on any site where you want the widget to appear.
            </CardDescription>
          </div>
          <CardAction>
            <Button variant="outline" size="sm" onClick={handleCopySnippet}>
              {snippetCopied ? (
                <>
                  <CheckIcon className="mr-2 size-4" />
                  Copied
                </>
              ) : (
                <>
                  <CopyIcon className="mr-2 size-4" />
                  Copy snippet
                </>
              )}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg border bg-muted/40 p-4 text-sm">
            <pre className="overflow-x-auto whitespace-pre-wrap font-mono text-xs leading-6 sm:text-sm">
              {embedSnippet}
            </pre>
          </div>
          <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
            <Badge variant="secondary" className="rounded-full px-3 py-1 text-[11px]">
              Served from {widgetScriptUrl}
            </Badge>
            <span>
              Keep the tenant key secret—treat it like a password and rotate it
              if it ever leaks.
            </span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1">
            <CardTitle>Tenant API tokens</CardTitle>
            <CardDescription>
              Tokens authenticate widget traffic. Only tenant owners can create
              or revoke them.
            </CardDescription>
          </div>
          <CardAction>
            <Button onClick={handleCreateKey} disabled={createKeyMutation.isPending}>
              {createKeyMutation.isPending ? (
                <>
                  <Loader2Icon className="mr-2 size-4 animate-spin" />
                  Issuing…
                </>
              ) : (
                <>
                  <RefreshCwIcon className="mr-2 size-4" />
                  Create new token
                </>
              )}
            </Button>
          </CardAction>
        </CardHeader>
        <Separator className="mx-6" />
        <CardContent className="space-y-3">
          {isLoading && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2Icon className="size-4 animate-spin" />
              Loading tokens…
            </div>
          )}

          {!isLoading && (apiKeys?.length ?? 0) === 0 && (
            <Empty className="border">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <ShieldCheckIcon className="size-5" />
                </EmptyMedia>
                <EmptyTitle>No tokens yet</EmptyTitle>
                <EmptyDescription>
                  Create a token to generate your first embed snippet. You can
                  rotate or revoke tokens at any time.
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={handleCreateKey} disabled={createKeyMutation.isPending}>
                  {createKeyMutation.isPending ? "Creating…" : "Create token"}
                </Button>
              </EmptyContent>
            </Empty>
          )}

          {(apiKeys?.length ?? 0) > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>API token</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {apiKeys?.map((key) => (
                  <TableRow key={key.keyId}>
                    <TableCell className="max-w-[320px] text-sm">
                      <div className="flex flex-col gap-1">
                        <code className="truncate rounded bg-muted px-2 py-1 font-mono text-xs">
                          {key.apiKey}
                        </code>
                        {activeKey === key.apiKey && (
                          <Badge variant="outline" className="w-fit text-[11px]">
                            Used in embed snippet
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatCreatedAt(key.createdAt)}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="gap-1"
                          onClick={() => handleCopyKeyValue(key.keyId, key.apiKey)}
                        >
                          {copiedKeyId === key.keyId ? (
                            <>
                              <CheckIcon className="size-4" />
                              Copied
                            </>
                          ) : (
                            <>
                              <CopyIcon className="size-4" />
                              Copy
                            </>
                          )}
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button
                              aria-label={`Delete token ${key.keyId}`}
                              variant="ghost"
                              size="icon"
                              disabled={deletingKeyId === key.keyId || deleteKeyMutation.isPending}
                            >
                              {deletingKeyId === key.keyId ? (
                                <Loader2Icon className="size-4 animate-spin" />
                              ) : (
                                <Trash2Icon className="size-4 text-destructive" />
                              )}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>Delete this token?</AlertDialogTitle>
                              <AlertDialogDescription>
                                Removing a token immediately invalidates traffic that relies on it.
                                You can always issue a new token afterwards.
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>Cancel</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() => handleDeleteKey(key.keyId)}
                                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                                disabled={deletingKeyId === key.keyId || deleteKeyMutation.isPending}
                              >
                                {deletingKeyId === key.keyId ? "Deleting…" : "Delete token"}
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableCaption className="text-left">
                Tokens are sorted by newest first. Rotate keys regularly and remove unused ones.
              </TableCaption>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function formatCreatedAt(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return "Not provided"
  }
  return format(parsed, "PPpp")
}
