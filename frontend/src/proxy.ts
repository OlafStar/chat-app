import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

const ACCESS_TOKEN_COOKIE = "pingy:access-token"

export function proxy(request: NextRequest) {
  const hasAccessToken = Boolean(request.cookies.get(ACCESS_TOKEN_COOKIE))
  if (!hasAccessToken) {
    const loginUrl = request.nextUrl.clone()
    loginUrl.pathname = "/login"
    loginUrl.searchParams.set("redirectedFrom", request.nextUrl.pathname)
    return NextResponse.redirect(loginUrl)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/dashboard/:path*"],
}
