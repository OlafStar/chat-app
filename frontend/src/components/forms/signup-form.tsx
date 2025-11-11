'use client'

import * as React from "react"
import { useRouter } from "next/navigation"

import { getAPIErrorMessage } from "~/lib/errors"
import { cn } from "~/lib/utils"
import { Button } from "~/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "~/components/ui/field"
import { Input } from "~/components/ui/input"
import Link from "next/link"
import { useSignupMutation } from "~/hooks/use-auth"

export function SignupForm({
  className,
  ...props
}: React.ComponentProps<"form">) {
  const [tenantName, setTenantName] = React.useState("")
  const [fullName, setFullName] = React.useState("")
  const [email, setEmail] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [confirmPassword, setConfirmPassword] = React.useState("")
  const [localError, setLocalError] = React.useState<string | null>(null)
  const router = useRouter()
  const signupMutation = useSignupMutation()

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    signupMutation.reset()

    if (password !== confirmPassword) {
      setLocalError("Passwords must match")
      return
    }

    setLocalError(null)
    signupMutation.mutate(
      {
        tenantName,
        name: fullName,
        email,
        password,
      },
      {
        onSuccess: () => {
          router.push("/dashboard")
        },
      }
    )
  }

  const errorMessage =
    localError ?? (signupMutation.error && getAPIErrorMessage(signupMutation.error))

  return (
    <form
      className={cn("flex flex-col gap-6", className)}
      onSubmit={handleSubmit}
      {...props}
    >
      <FieldGroup>
        <div className="flex flex-col items-center gap-1 text-center">
          <h1 className="text-2xl font-bold">Create your account</h1>
          <p className="text-muted-foreground text-sm text-balance">
            Fill in the form below to create your account
          </p>
        </div>
        <Field>
          <FieldLabel htmlFor="tenant-name">Workspace name</FieldLabel>
          <Input
            id="tenant-name"
            type="text"
            placeholder="Acme Engineering"
            required
            value={tenantName}
            onChange={(event) => setTenantName(event.target.value)}
          />
          <FieldDescription>
            This will be used as the tenant name for your workspace.
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="name">Full name</FieldLabel>
          <Input
            id="name"
            type="text"
            placeholder="John Doe"
            required
            autoComplete="name"
            value={fullName}
            onChange={(event) => setFullName(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="email">Email</FieldLabel>
          <Input
            id="email"
            type="email"
            placeholder="m@example.com"
            required
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
          <FieldDescription>
            We&apos;ll use this to contact you. We will not share your email
            with anyone else.
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="password">Password</FieldLabel>
          <Input
            id="password"
            type="password"
            required
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <FieldDescription>
            Must be at least 8 characters long.
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="confirm-password">Confirm password</FieldLabel>
          <Input
            id="confirm-password"
            type="password"
            required
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
          />
          <FieldDescription>Please confirm your password.</FieldDescription>
        </Field>
        <Field>
          <Button type="submit" disabled={signupMutation.isPending}>
            {signupMutation.isPending ? "Creating account…" : "Create account"}
          </Button>
        </Field>
        {errorMessage && (
          <FieldDescription className="text-center text-destructive">
            {errorMessage}
          </FieldDescription>
        )}
        <Field>
          <FieldDescription className="px-6 text-center">
            Already have an account?{" "}
            <Link
              href="/login"
              className="underline underline-offset-4 hover:text-primary-foreground"
            >
              Sign in
            </Link>
          </FieldDescription>
        </Field>
      </FieldGroup>
    </form>
  )
}
