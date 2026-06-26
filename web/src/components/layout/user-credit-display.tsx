"use client";

import Link from "next/link";
import { Zap } from "lucide-react";
import { getStoredToken } from "@/services/api/client";
import { useUserCreditBalance } from "@/constant/credits";

export function UserCreditDisplay() {
  const token = getStoredToken();
  const balance = useUserCreditBalance();

  if (!token || balance === null) return null;

  return (
    <Link href="/credits" className="inline-flex items-center gap-1 text-xs text-stone-500 hover:text-amber-600 dark:text-stone-400 dark:hover:text-amber-400 transition-colors" title="积分明细">
      <Zap className="size-3 fill-amber-400 text-amber-400" />
      <span>{balance.toLocaleString()}</span>
    </Link>
  );
}
