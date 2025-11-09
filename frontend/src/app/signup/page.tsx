import type { Metadata } from "next";
import Link from "next/link";

import { AuthShell } from "~/components/auth/auth-shell";
import { SignupForm } from "~/components/auth/signup-form";

export const metadata: Metadata = {
  title: "Pingy Console | Create workspace",
  description: "Launch a Pingy tenant for your support organization.",
};

export default function SignupPage() {
  return (
    <AuthShell
      title="Create your Pingy workspace"
      subtitle="Stand up a secure tenant for your agents and automate the boring parts of support."
      footer={
        <>
          Already have a login?{" "}
          <Link className="font-medium text-primary underline" href="/login">
            Go to sign in
          </Link>
        </>
      }
    >
      <SignupForm />
    </AuthShell>
  );
}
