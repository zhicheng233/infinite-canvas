import clsx from "clsx";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export type MarkdownContentProps = {
    content: string;
    className?: string;
};

function isExternalLink(href?: string) {
    return Boolean(href && /^(?:https?:)?\/\//i.test(href));
}

export function MarkdownContent({ content, className }: MarkdownContentProps) {
    return (
        <div className={clsx("min-w-0 text-sm leading-7 text-foreground", className)}>
            <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                skipHtml
                components={{
                    h1: ({ node: _, ...props }) => <h1 className="mb-4 mt-6 text-2xl font-semibold first:mt-0" {...props} />,
                    h2: ({ node: _, ...props }) => <h2 className="mb-3 mt-6 text-xl font-semibold first:mt-0" {...props} />,
                    h3: ({ node: _, ...props }) => <h3 className="mb-2 mt-5 text-base font-semibold first:mt-0" {...props} />,
                    p: ({ node: _, ...props }) => <p className="my-3 first:mt-0 last:mb-0" {...props} />,
                    ul: ({ node: _, ...props }) => <ul className="my-3 list-disc space-y-1 pl-6" {...props} />,
                    ol: ({ node: _, ...props }) => <ol className="my-3 list-decimal space-y-1 pl-6" {...props} />,
                    blockquote: ({ node: _, ...props }) => <blockquote className="my-4 border-l-2 border-border pl-4 text-muted-foreground" {...props} />,
                    hr: ({ node: _, ...props }) => <hr className="my-5 border-border" {...props} />,
                    a: ({ node: _, href, ...props }) => {
                        const external = isExternalLink(href);
                        return <a {...props} className="font-medium underline underline-offset-4" href={href} target={external ? "_blank" : undefined} rel={external ? "noopener noreferrer" : undefined} />;
                    },
                    img: ({ node: _, className: imageClassName, ...props }) => <img {...props} className={clsx("my-4 h-auto max-w-full rounded-md", imageClassName)} loading="lazy" />,
                    pre: ({ node: _, ...props }) => <pre className="my-4 max-w-full overflow-x-auto rounded-md bg-muted p-4 text-xs leading-6 [&>code]:bg-transparent [&>code]:p-0" {...props} />,
                    code: ({ node: _, className: codeClassName, ...props }) => <code className={clsx("rounded bg-muted px-1 py-0.5 font-mono text-[0.9em]", codeClassName)} {...props} />,
                    table: ({ node: _, ...props }) => (
                        <div className="my-4 max-w-full overflow-x-auto">
                            <table className="min-w-full border-collapse text-left text-sm" {...props} />
                        </div>
                    ),
                    th: ({ node: _, ...props }) => <th className="border border-border bg-muted px-3 py-2 font-semibold" {...props} />,
                    td: ({ node: _, ...props }) => <td className="border border-border px-3 py-2 align-top" {...props} />,
                }}
            >
                {content}
            </ReactMarkdown>
        </div>
    );
}
