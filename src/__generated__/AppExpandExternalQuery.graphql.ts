/**
 * @generated SignedSource<<d7ba5f1114aaf7cbd041981c7efe6448>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type EdgeKind = "IMPORTS" | "USED_BY" | "USES" | "%future added value";
export type NodeKind = "ATOM" | "FILE" | "MODULE" | "%future added value";
export type AppExpandExternalQuery$variables = {
  focus: string;
};
export type AppExpandExternalQuery$data = {
  readonly neighborhood: {
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
};
export type AppExpandExternalQuery = {
  response: AppExpandExternalQuery$data;
  variables: AppExpandExternalQuery$variables;
};

const node: ConcreteRequest = (function(){
var v0 = [
  {
    "defaultValue": null,
    "kind": "LocalArgument",
    "name": "focus"
  }
],
v1 = {
  "alias": null,
  "args": null,
  "kind": "ScalarField",
  "name": "kind",
  "storageKey": null
},
v2 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "id",
    "storageKey": null
  },
  (v1/*: any*/),
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
v3 = [
  {
    "alias": null,
    "args": [
      {
        "kind": "Variable",
        "name": "ref",
        "variableName": "focus"
      }
    ],
    "concreteType": "Neighborhood",
    "kind": "LinkedField",
    "name": "neighborhood",
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
        "selections": (v2/*: any*/),
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "concreteType": "GraphNode",
        "kind": "LinkedField",
        "name": "nodes",
        "plural": true,
        "selections": (v2/*: any*/),
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
          (v1/*: any*/)
        ],
        "storageKey": null
      }
    ],
    "storageKey": null
  }
];
return {
  "fragment": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Fragment",
    "metadata": null,
    "name": "AppExpandExternalQuery",
    "selections": (v3/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Operation",
    "name": "AppExpandExternalQuery",
    "selections": (v3/*: any*/)
  },
  "params": {
    "cacheID": "2baae248b415e9736cfdfafcc5380110",
    "id": null,
    "metadata": {},
    "name": "AppExpandExternalQuery",
    "operationKind": "query",
    "text": "query AppExpandExternalQuery(\n  $focus: ID!\n) {\n  neighborhood(ref: $focus) {\n    incomplete\n    focus {\n      id\n      kind\n      label\n      parentId\n      external\n      expandable\n    }\n    nodes {\n      id\n      kind\n      label\n      parentId\n      external\n      expandable\n    }\n    edges {\n      from\n      to\n      kind\n    }\n  }\n}\n"
  }
};
})();

(node as any).hash = "ea4a1b1fa0b79f3298d971c032e388f5";

export default node;
