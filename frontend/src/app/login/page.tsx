import type { Metadata } from "next";
import Link from "next/link";

import { AuthShell } from "~/components/auth/auth-shell";
import { LoginForm } from "~/components/auth/login-form";

export const metadata: Metadata = {
  title: "Pingy Console | Login",
  description: "Access the Pingy workspace dashboard.",
};

export default function LoginPage() {
  return (
    <AuthShell
      title="Welcome back"
      subtitle="Log in to collaborate with your team and support visitors in real time."
      footer={
        <>
          Need an invite?{" "}
          <Link className="font-medium text-primary underline" href="/signup">
            Create a workspace
          </Link>
        </>
      }
    >
      <LoginForm />
    </AuthShell>
  );
}
