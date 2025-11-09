"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { useState } from "react";

import { registerTenant, type RegisterRequest } from "~/lib/api/auth";
import { APIError } from "~/lib/api/client";
import { persistAuthSession } from "~/lib/auth/session";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";

const initialState: RegisterRequest = {
  tenantName: "",
  name: "",
  email: "",
  password: "",
};

export function SignupForm() {
  const router = useRouter();
  const [form, setForm] = useState(initialState);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    setError(null);
    setSuccess(null);

    try {
      const payload = await registerTenant(form);
      persistAuthSession(payload);
      setSuccess("Workspace created! Redirecting...");
      router.push("/dashboard");
    } catch (err) {
      const message =
        err instanceof APIError
          ? err.message
          : "Unable to create your workspace right now.";
      setError(message);
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Card className="border-border/70 shadow-lg">
      <CardHeader className="space-y-1">
        <CardTitle className="text-2xl">Create your workspace</CardTitle>
        <CardDescription>
          We will spin up your tenant, owner account, and a starter API key.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-5" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="tenantName">Workspace name</Label>
            <Input
              id="tenantName"
              placeholder="Pingy HQ"
              required
              value={form.tenantName}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  tenantName: event.target.value,
                }))
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="name">Your name</Label>
            <Input
              id="name"
              placeholder="Ada Lovelace"
              required
              value={form.name}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  name: event.target.value,
                }))
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="email">Work email</Label>
            <Input
              id="email"
              type="email"
              placeholder="team@pingy.com"
              autoComplete="email"
              required
              value={form.email}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  email: event.target.value,
                }))
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              autoComplete="new-password"
              required
              value={form.password}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  password: event.target.value,
                }))
              }
            />
            <p className="text-xs text-muted-foreground">
              Minimum 10 characters. Choose something unique to your account.
            </p>
          </div>

          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          {success ? (
            <Alert>
              <AlertDescription>{success}</AlertDescription>
            </Alert>
          ) : null}

          <Button className="w-full" disabled={isSubmitting} type="submit">
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Creating workspace...
              </>
            ) : (
              "Create workspace"
            )}
          </Button>
        </form>
      </CardContent>
      <CardFooter className="justify-center text-sm text-muted-foreground">
        Already onboarded?&nbsp;
        <Link className="font-medium text-primary underline" href="/login">
          Go to login
        </Link>
      </CardFooter>
    </Card>
  );
}
