"use client";

import { Loader2, LogOut } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Separator } from "~/components/ui/separator";
import {
  clearAuthSession,
  loadAuthSession,
  type AuthSession,
} from "~/lib/auth/session";

export default function DashboardPage() {
  const router = useRouter();
  const [session, setSession] = useState<AuthSession | null>(null);
  const [isChecking, setIsChecking] = useState(true);

  useEffect(() => {
    const stored = loadAuthSession();
    if (!stored) {
      router.replace("/login");
      return;
    }
    setSession(stored);
    setIsChecking(false);
  }, [router]);

  function handleSignOut() {
    clearAuthSession();
    router.replace("/login");
  }

  if (isChecking) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">
        <Loader2 className="h-6 w-6 animate-spin" />
      </div>
    );
  }

  if (!session) {
    return null;
  }

  return (
    <main className="min-h-screen bg-muted/30">
      <div className="mx-auto flex max-w-4xl flex-col gap-6 px-6 py-16">
        <div className="flex flex-col gap-3 text-center">
          <h1 className="text-3xl font-semibold tracking-tight">
            {session.tenant.name}
          </h1>
          <p className="text-muted-foreground">
            Signed in as {session.user.name} ({session.user.email})
          </p>
        </div>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle>Active tenant</CardTitle>
              <CardDescription>
                Keep this tab open while building the rest of the dashboard.
              </CardDescription>
            </div>
            <Button onClick={handleSignOut} variant="outline">
              <LogOut className="mr-2 h-4 w-4" />
              Sign out
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            <div>
              <p className="text-sm font-medium text-muted-foreground">
                Tenant ID
              </p>
              <p className="text-base font-semibold">{session.tenant.tenantId}</p>
            </div>
            <div className="grid gap-4 rounded-lg border bg-muted/30 p-4 sm:grid-cols-3">
              <div>
                <p className="text-xs uppercase text-muted-foreground">Plan</p>
                <p className="text-base font-medium">{session.tenant.plan}</p>
              </div>
              <div>
                <p className="text-xs uppercase text-muted-foreground">Seats</p>
                <p className="text-base font-medium">{session.tenant.seats}</p>
              </div>
              <div>
                <p className="text-xs uppercase text-muted-foreground">
                  Created
                </p>
                <p className="text-base font-medium">
                  {new Date(session.tenant.createdAt).toLocaleString()}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {session.tenants && session.tenants.length > 0 ? (
          <Card>
            <CardHeader>
              <CardTitle>Workspace memberships</CardTitle>
              <CardDescription>
                Switching tenants will refresh your access token.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {session.tenants.map((membership, index) => (
                <div key={membership.tenantId}>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <p className="font-medium">{membership.name}</p>
                      <p className="text-sm text-muted-foreground">
                        Role: {membership.role} | Status: {membership.status}
                      </p>
                    </div>
                    {membership.isDefault ? (
                      <span className="text-xs font-semibold uppercase text-primary">
                        Default
                      </span>
                    ) : null}
                  </div>
                  {index < session.tenants!.length - 1 ? (
                    <Separator className="my-3" />
                  ) : null}
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}
      </div>
    </main>
  );
}
