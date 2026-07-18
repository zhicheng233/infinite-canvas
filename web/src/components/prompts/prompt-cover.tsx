"use client";

import { ImageOff } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

export function PromptCover({ src, title, className, imgClassName }: { src: string; title: string; className?: string; imgClassName?: string }) {
    const [failed, setFailed] = useState(false);

    useEffect(() => {
        setFailed(false);
    }, [src]);

    if (!src || failed) {
        return (
            <div className={cn("flex items-center justify-center bg-gradient-to-br from-stone-200 via-stone-100 to-stone-50 text-stone-500 dark:from-stone-900 dark:via-stone-950 dark:to-black dark:text-stone-400", className)}>
                <div className="flex max-w-[80%] flex-col items-center gap-3 text-center">
                    <ImageOff className="size-6 opacity-70" />
                    <div className="line-clamp-3 text-sm leading-6">{title}</div>
                </div>
            </div>
        );
    }

    return <img src={src} alt={title} className={cn(className, imgClassName)} loading="lazy" referrerPolicy="no-referrer" onError={() => setFailed(true)} />;
}
