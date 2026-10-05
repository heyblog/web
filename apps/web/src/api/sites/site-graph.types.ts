export interface GraphNode {
  readonly id: string;
  readonly name: string;
  readonly host: string;
  readonly homepageUrl: string;
  readonly shortId: string | null;
  readonly customId: string | null;
}

export interface GraphEdge {
  readonly source: string;
  readonly target: string;
  readonly reciprocal: boolean;
}

export interface SiteGraph {
  readonly nodes: readonly GraphNode[];
  readonly edges: readonly GraphEdge[];
  readonly stats: {
    readonly nodes: number;
    readonly edges: number;
    readonly reciprocalPairs: number;
  };
  readonly centerId: string | null;
}
