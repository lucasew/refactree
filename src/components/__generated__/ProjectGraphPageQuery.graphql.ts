/**
 * @generated SignedSource<<8365b4af3ea9802ef3ad2e2fb770729e>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type EdgeKind = "IMPORTS" | "USED_BY" | "USES" | "%future added value";
export type NodeKind = "ATOM" | "FILE" | "MODULE" | "%future added value";
export type ProjectGraphPageQuery$variables = Record<PropertyKey, never>;
export type ProjectGraphPageQuery$data = {
  readonly projectGraph: {
    readonly edges: ReadonlyArray<{
      readonly from: string;
      readonly kind: EdgeKind;
      readonly to: string;
    }>;
    readonly focus: {
      readonly expandable: boolean;
      readonly external: boolean;
      readonly id: string;
      readonly kind: NodeKind;
      readonly label: string;
      readonly parentId: string | null | undefined;
    };
    readonly incomplete: boolean;
    readonly nodes: ReadonlyArray<{
      readonly expandable: boolean;
      readonly external: boolean;
      readonly id: string;
      readonly kind: NodeKind;
      readonly label: string;
      readonly parentId: string | null | undefined;
    }>;
  };
  readonly rootDir: string;
};
export type ProjectGraphPageQuery = {
  response: ProjectGraphPageQuery$data;
  variables: ProjectGraphPageQuery$variables;
};

const node: ConcreteRequest = (function(){
var v0 = {
  "alias": null,
  "args": null,
  "kind": "ScalarField",
  "name": "kind",
  "storageKey": null
},
v1 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "id",
    "storageKey": null
  },
  (v0/*: any*/),
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "label",
    "storageKey": null
  },
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "parentId",
    "storageKey": null
  },
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "external",
    "storageKey": null
  },
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "expandable",
    "storageKey": null
  }
],
v2 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "rootDir",
    "storageKey": null
  },
  {
    "alias": null,
    "args": null,
    "concreteType": "Neighborhood",
    "kind": "LinkedField",
    "name": "projectGraph",
    "plural": false,
    "selections": [
      {
        "alias": null,
        "args": null,
        "kind": "ScalarField",
        "name": "incomplete",
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "concreteType": "GraphNode",
        "kind": "LinkedField",
        "name": "focus",
        "plural": false,
        "selections": (v1/*: any*/),
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "concreteType": "GraphNode",
        "kind": "LinkedField",
        "name": "nodes",
        "plural": true,
        "selections": (v1/*: any*/),
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "concreteType": "GraphEdge",
        "kind": "LinkedField",
        "name": "edges",
        "plural": true,
        "selections": [
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "from",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "to",
            "storageKey": null
          },
          (v0/*: any*/)
        ],
        "storageKey": null
      }
    ],
    "storageKey": null
  }
];
return {
  "fragment": {
    "argumentDefinitions": [],
    "kind": "Fragment",
    "metadata": null,
    "name": "ProjectGraphPageQuery",
    "selections": (v2/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": [],
    "kind": "Operation",
    "name": "ProjectGraphPageQuery",
    "selections": (v2/*: any*/)
  },
  "params": {
    "cacheID": "deebf0dd416c1e334f41b944ffb8d60e",
    "id": null,
    "metadata": {},
    "name": "ProjectGraphPageQuery",
    "operationKind": "query",
    "text": "query ProjectGraphPageQuery {\n  rootDir\n  projectGraph {\n    incomplete\n    focus {\n      id\n      kind\n      label\n      parentId\n      external\n      expandable\n    }\n    nodes {\n      id\n      kind\n      label\n      parentId\n      external\n      expandable\n    }\n    edges {\n      from\n      to\n      kind\n    }\n  }\n}\n"
  }
};
})();

(node as any).hash = "3090f1f628f211b6b25a26e25078c6e8";

export default node;
