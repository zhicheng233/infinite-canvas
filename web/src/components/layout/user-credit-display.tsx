"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Zap } from "lucide-react";
import { getBalance } from "@/services/api/credits";
import { getStoredToken } from "@/services/api/client";

export function UserCreditDisplay() {
  const [balance, setBalance] = useState<number | null>(null);
  const token = getStoredToken();

  useEffect(() => {
    if (!token) return;
    getBalance().then((data) => setBalance(data.balance)).catch(() => {});
  }, [token]);

  if (!token || balance === null) return null;

  return (
    <Link href="/recharge" className="inline-flex items-center gap-1 text-xs text-stone-500 hover:text-amber-600 dark:text-stone-400 dark:hover:text-amber-400 transition-colors" title="积分充值">
      <Zap className="size-3 fill-amber-400 text-amber-400" />
      <span>{balance.toLocaleString()}</span>
    </Link>
  );
}
