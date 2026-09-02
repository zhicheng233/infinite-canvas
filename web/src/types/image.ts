export type ReferenceImage = {
    id: string;
    name: string;
    type: string;
    dataUrl: string;
    url?: string;
    storageKey?: string;
    generationRequestId?: string;
    generationCost?: number;
    resolvedChannelName?: string;
};
