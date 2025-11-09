import Link from "next/link";
import type { ReactNode } from "react";

import { cn } from "~/lib/utils";
import { Separator } from "~/components/ui/separator";

type Callout = {
  heading: string;
  body: string;
  highlight?: string;
};

type AuthShellProps = {
  children: ReactNode;
  title: string;
  subtitle: string;
  footer?: ReactNode;
  callout?: Callout;
  supportLabel?: string;
  supportHref?: string;
  supportLinkLabel?: string;
  className?: string;
};

export function AuthShell({
  children,
  title,
  subtitle,
  footer,
  callout = {
    heading: "Chat without compromises",
    body: "Design personalized support journeys powered by a modern agent console.",
    highlight: "Pingy helps teams move faster.",
  },
  supportLabel = "Need help?",
  supportHref = "/contact",
  supportLinkLabel = "Contact us",
  className,
}: AuthShellProps) {
  return (
    <div className="grid min-h-screen w-full lg:grid-cols-2">
      <aside className="relative hidden overflow-hidden border-r bg-muted/40 lg:flex">
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-primary/80 via-primary/40 to-primary" />
        <div className="relative z-10 flex h-full w-full flex-col justify-between px-10 py-12 text-primary-foreground">
          {callout.highlight ? (
            <div className="text-sm uppercase tracking-wide text-white/80">
              {callout.highlight}
            </div>
          ) : (
            <div />
          )}
          <div>
            <p className="text-lg font-medium uppercase tracking-tight text-white/90">
              {supportLabel} -{" "}
              <Link className="underline" href={supportHref}>
                {supportLinkLabel}
              </Link>
            </p>
            <Separator className="my-6 border-white/20 bg-white/20" />
            <h2 className="text-4xl font-semibold leading-tight">
              {callout.heading}
            </h2>
            <p className="mt-4 max-w-md text-base text-white/80">
              {callout.body}
            </p>
          </div>
        </div>
      </aside>

      <div
        className={cn(
          "flex flex-1 items-center justify-center px-6 py-12 sm:px-8",
          className,
        )}
      >
        <div className="w-full max-w-md space-y-6">
          <div className="space-y-2 text-center">
            <p className="text-sm uppercase tracking-wider text-muted-foreground">
              Pingy Support OS
            </p>
            <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          </div>
          {children}
          {footer ? (
            <div className="text-center text-sm text-muted-foreground">
              {footer}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
