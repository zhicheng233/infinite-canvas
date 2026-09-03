import { beforeEach, describe, expect, it, jest, mock } from "bun:test";
import { createElement, type ReactNode } from "react";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import type { SiteAnnouncement } from "@/services/api/announcement";
import type { FeatureGuide, FeatureGuideSurface } from "@/services/api/feature-guide";

type MockProps = Record<string, unknown> & { children?: ReactNode };

let pathname = "/canvas/project-id";
let user = { id: "user-a" };
const getAnnouncement = jest.fn<() => Promise<SiteAnnouncement>>();
const getFeatureGuide = jest.fn<(surface: FeatureGuideSurface) => Promise<FeatureGuide | null>>();
const storageValues = new Map<string, string>();
let storageReadFails = false;
let storageWriteFails = false;

function MockComponent({ children }: MockProps) {
    return createElement("mock-component", null, children);
}

function MockModal({ children, footer, ...props }: MockProps) {
    return createElement("announcement-modal", props, children, footer as ReactNode);
}

function MockButton(props: MockProps) {
    return createElement("announcement-button", props, props.children);
}

function MockFeatureGuideDialog(props: MockProps) {
    return createElement("feature-guide-dialog", props);
}

mock.module("next/navigation", () => ({ usePathname: () => pathname }));
mock.module("antd", () => ({
    Button: MockButton,
    Checkbox: MockComponent,
    Modal: MockModal,
    Typography: { Paragraph: MockComponent },
}));
mock.module("lucide-react", () => ({ Megaphone: MockComponent }));
mock.module("@/services/api/announcement", () => ({ getAnnouncement }));
mock.module("@/services/api/feature-guide", () => ({ getFeatureGuide }));
mock.module("@/stores/use-user-store", () => ({ useUserStore: (selector: (state: { user: typeof user }) => unknown) => selector({ user }) }));
mock.module("@/components/feature-guide/feature-guide-dialog", () => ({ FeatureGuideDialog: MockFeatureGuideDialog }));

const { GlobalAnnouncementDialog } = await import("./global-announcement-dialog");

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });
Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
        getItem: jest.fn((key: string) => {
            if (storageReadFails) throw new Error("storage read failed");
            return storageValues.get(key) ?? null;
        }),
        setItem: jest.fn((key: string, value: string) => {
            if (storageWriteFails) throw new Error("storage write failed");
            storageValues.set(key, value);
        }),
    },
});

const announcement: SiteAnnouncement = { enabled: true, title: "公告", content: "公告内容", version: 2 };
const guides: Record<FeatureGuideSurface, FeatureGuide> = {
    canvas: { surface: "canvas", enabled: true, title: "画布引导", pages: ["画布内容"], version: 3 },
    image: { surface: "image", enabled: true, title: "图片引导", pages: ["图片内容"], version: 4 },
    video: { surface: "video", enabled: true, title: "视频引导", pages: ["视频内容"], version: 5 },
};

function deferred<T>() {
    let resolve: (value: T) => void = () => undefined;
    const promise = new Promise<T>((resolvePromise) => {
        resolve = resolvePromise;
    });
    return { promise, resolve };
}

async function renderCoordinator() {
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<GlobalAnnouncementDialog />);
    });
    if (!renderer) throw new Error("global announcement dialog did not render");
    return renderer;
}

beforeEach(() => {
    pathname = "/canvas/project-id";
    user = { id: "user-a" };
    storageValues.clear();
    storageReadFails = false;
    storageWriteFails = false;
    getAnnouncement.mockReset();
    getFeatureGuide.mockReset();
    getAnnouncement.mockResolvedValue(announcement);
    getFeatureGuide.mockImplementation(async (surface) => guides[surface]);
});

describe("GlobalAnnouncementDialog", () => {
    it("keeps an unread guide hidden until the unread announcement finishes closing", async () => {
        const renderer = await renderCoordinator();
        const modal = renderer.root.findByType(MockModal);

        expect(modal.props.open).toBe(true);
        expect(renderer.root.findAllByType(MockFeatureGuideDialog)).toHaveLength(0);

        await act(async () => modal.props.onCancel());
        expect(renderer.root.findByType(MockModal).props.open).toBe(false);
        expect(renderer.root.findAllByType(MockFeatureGuideDialog)).toHaveLength(0);

        await act(async () => {
            renderer.root.findByType(MockModal).props.afterOpenChange(false);
        });
        expect(renderer.root.findByType(MockFeatureGuideDialog).props.guide).toEqual(guides.canvas);
        await act(async () => renderer.unmount());
    });

    it("writes the account and surface completion key before closing the guide", async () => {
        getAnnouncement.mockResolvedValue({ ...announcement, enabled: false });
        const renderer = await renderCoordinator();
        const dialog = renderer.root.findByType(MockFeatureGuideDialog);

        await act(async () => dialog.props.onComplete());
        expect(storageValues.get("infinite-canvas:feature-guide:user-a:canvas:completed-version")).toBe("3");
        expect(renderer.root.findAllByType(MockFeatureGuideDialog)).toHaveLength(0);
        await act(async () => renderer.unmount());
    });

    it("still closes when storage reads or writes fail", async () => {
        storageReadFails = true;
        storageWriteFails = true;
        const renderer = await renderCoordinator();
        const modal = renderer.root.findByType(MockModal);

        await act(async () => {
            expect(() => modal.props.onCancel()).not.toThrow();
        });
        expect(renderer.root.findByType(MockModal).props.open).toBe(false);
        await act(async () => renderer.root.findByType(MockModal).props.afterOpenChange(false));

        const dialog = renderer.root.findByType(MockFeatureGuideDialog);
        await act(async () => {
            expect(() => dialog.props.onComplete()).not.toThrow();
        });
        expect(renderer.root.findAllByType(MockFeatureGuideDialog)).toHaveLength(0);
        await act(async () => renderer.unmount());
    });

    it("remembers a failed completion write for this app session while isolating accounts and versions", async () => {
        getAnnouncement.mockResolvedValue({ ...announcement, enabled: false });
        storageWriteFails = true;
        let canvasVersion = 3;
        getFeatureGuide.mockImplementation(async (surface) => (surface === "canvas" ? { ...guides.canvas, version: canvasVersion } : guides[surface]));
        const renderer = await renderCoordinator();

        await act(async () => renderer.root.findByType(MockFeatureGuideDialog).props.onComplete());
        pathname = "/settings";
        await act(async () => renderer.update(<GlobalAnnouncementDialog />));
        pathname = "/canvas/project-id";
        await act(async () => renderer.update(<GlobalAnnouncementDialog />));
        expect(renderer.root.findAllByType(MockFeatureGuideDialog)).toHaveLength(0);

        user = { id: "user-b" };
        await act(async () => renderer.update(<GlobalAnnouncementDialog />));
        expect(renderer.root.findByType(MockFeatureGuideDialog).props.guide.version).toBe(3);

        await act(async () => renderer.root.findByType(MockFeatureGuideDialog).props.onComplete());
        pathname = "/settings";
        await act(async () => renderer.update(<GlobalAnnouncementDialog />));
        canvasVersion = 4;
        pathname = "/canvas/project-id";
        await act(async () => renderer.update(<GlobalAnnouncementDialog />));
        expect(renderer.root.findByType(MockFeatureGuideDialog).props.guide.version).toBe(4);
        await act(async () => renderer.unmount());
    });

    it("ignores guide results from an older surface and user", async () => {
        getAnnouncement.mockResolvedValue({ ...announcement, enabled: false });
        const oldRequest = deferred<FeatureGuide | null>();
        const currentRequest = deferred<FeatureGuide | null>();
        getFeatureGuide.mockImplementationOnce(() => oldRequest.promise).mockImplementationOnce(() => currentRequest.promise);
        const renderer = await renderCoordinator();

        pathname = "/image";
        user = { id: "user-b" };
        await act(async () => renderer.update(<GlobalAnnouncementDialog />));

        await act(async () => {
            oldRequest.resolve(guides.canvas);
            await oldRequest.promise;
        });
        expect(renderer.root.findAllByType(MockFeatureGuideDialog)).toHaveLength(0);

        await act(async () => {
            currentRequest.resolve(guides.image);
            await currentRequest.promise;
        });
        expect(renderer.root.findByType(MockFeatureGuideDialog).props.guide).toEqual(guides.image);
        await act(async () => renderer.unmount());
    });
});
