import { NextResponse } from "next/server";

import { buildVersion } from "@/lib/build-version";

export function GET() {
    return NextResponse.json({ service: "frontend", status: "ok", version: buildVersion });
}
