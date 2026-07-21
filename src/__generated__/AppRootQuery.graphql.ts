/**
 * @generated SignedSource<<a474672090243a3e1e21e116553a7af3>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type EdgeKind = "IMPORTS" | "USED_BY" | "USES" | "%future added value";
export type NodeKind = "ATOM" | "FILE" | "MODULE" | "%future added value";
export type AppRootQuery$variables = {
  focus: string;
  fsRef?: string | null | undefined;
  hasFocus: boolean;
};
export type AppRootQuery$data = {
  readonly code?: {
    readonly error: string | null | undefined;
    readonly files: ReadonlyArray<{
      readonly isDir: boolean;
      readonly name: string;
      readonly reference: string;
    }>;
    readonly focusId: string | null | undefined;
    readonly language: string | null | undefined;
    readonly nonText: boolean;
    readonly parentHref: string | null | undefined;
    readonly reference: string;
    readonly segments: ReadonlyArray<{
      readonly anchorId: string | null | undefined;
      readonly href: string | null | undefined;
      readonly isDef: boolean;
      readonly isLink: boolean;
      readonly reference: string | null | undefined;
      readonly text: string;
    }>;
    readonly symbols: ReadonlyArray<{
      readonly isDir: boolean;
      readonly name: string;
      readonly reference: string;
    }>;
    readonly warning: string | null | undefined;
  };
  readonly filesystem: ReadonlyArray<{
    readonly isDir: boolean;
    readonly name: string;
    readonly reference: string;
  }>;
  readonly neighborhood?: {
    readonly edges: ReadonlyArray<{
      readonly from: string;
      readonly kind: EdgeKind;
      readonly to: string;
    }>;
    readonly focus: {
      readonly id: string;
      readonly kind: NodeKind;
      readonly label: string;
      readonly parentId: string | null | undefined;
    };
    readonly incomplete: boolean;
    readonly nodes: ReadonlyArray<{
      readonly id: string;
      readonly kind: NodeKind;
      readonly label: string;
      readonly parentId: string | null | undefined;
    }>;
  };
  readonly rootDir: string;
};
export type AppRootQuery = {
  response: AppRootQuery$data;
  variables: AppRootQuery$variables;
};

const node: ConcreteRequest = (function(){
var v0 = {
  "defaultValue": null,
  "kind": "LocalArgument",
  "name": "focus"
},
v1 = {
  "defaultValue": null,
  "kind": "LocalArgument",
  "name": "fsRef"
},
v2 = {
  "defaultValue": null,
  "kind": "LocalArgument",
  "name": "hasFocus"
},
v3 = {
  "alias": null,
  "args": null,
  "kind": "ScalarField",
  "name": "reference",
  "storageKey": null
},
v4 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "name",
    "storageKey": null
  },
  (v3/*: any*/),
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "isDir",
    "storageKey": null
  }
],
v5 = [
  {
    "kind": "Variable",
    "name": "ref",
    "variableName": "focus"
  }
],
v6 = {
  "alias": null,
  "args": null,
  "kind": "ScalarField",
  "name": "kind",
  "storageKey": null
},
v7 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "id",
    "storageKey": null
  },
  (v6/*: any*/),
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
  }
],
v8 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "rootDir",
    "storageKey": null
  },
  {
    "alias": null,
    "args": [
      {
        "kind": "Variable",
        "name": "ref",
        "variableName": "fsRef"
      }
    ],
    "concreteType": "FsEntry",
    "kind": "LinkedField",
    "name": "filesystem",
    "plural": true,
    "selections": (v4/*: any*/),
    "storageKey": null
  },
  {
    "condition": "hasFocus",
    "kind": "Condition",
    "passingValue": true,
    "selections": [
      {
        "alias": null,
        "args": (v5/*: any*/),
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
            "selections": (v7/*: any*/),
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "concreteType": "GraphNode",
            "kind": "LinkedField",
            "name": "nodes",
            "plural": true,
            "selections": (v7/*: any*/),
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
              (v6/*: any*/)
            ],
            "storageKey": null
          }
        ],
        "storageKey": null
      },
      {
        "alias": null,
        "args": (v5/*: any*/),
        "concreteType": "CodeDocument",
        "kind": "LinkedField",
        "name": "code",
        "plural": false,
        "selections": [
          (v3/*: any*/),
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "language",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "nonText",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "error",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "warning",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "focusId",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "parentHref",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "concreteType": "CodeSegment",
            "kind": "LinkedField",
            "name": "segments",
            "plural": true,
            "selections": [
              {
                "alias": null,
                "args": null,
                "kind": "ScalarField",
                "name": "text",
                "storageKey": null
              },
              {
                "alias": null,
                "args": null,
                "kind": "ScalarField",
                "name": "href",
                "storageKey": null
              },
              {
                "alias": null,
                "args": null,
                "kind": "ScalarField",
                "name": "anchorId",
                "storageKey": null
              },
              {
                "alias": null,
                "args": null,
                "kind": "ScalarField",
                "name": "isLink",
                "storageKey": null
              },
              {
                "alias": null,
                "args": null,
                "kind": "ScalarField",
                "name": "isDef",
                "storageKey": null
              },
              (v3/*: any*/)
            ],
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "concreteType": "FsEntry",
            "kind": "LinkedField",
            "name": "files",
            "plural": true,
            "selections": (v4/*: any*/),
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "concreteType": "FsEntry",
            "kind": "LinkedField",
            "name": "symbols",
            "plural": true,
            "selections": (v4/*: any*/),
            "storageKey": null
          }
        ],
        "storageKey": null
      }
    ]
  }
];
return {
  "fragment": {
    "argumentDefinitions": [
      (v0/*: any*/),
      (v1/*: any*/),
      (v2/*: any*/)
    ],
    "kind": "Fragment",
    "metadata": null,
    "name": "AppRootQuery",
    "selections": (v8/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": [
      (v1/*: any*/),
      (v0/*: any*/),
      (v2/*: any*/)
    ],
    "kind": "Operation",
    "name": "AppRootQuery",
    "selections": (v8/*: any*/)
  },
  "params": {
    "cacheID": "f01d36005ae166bd97505b6dbc7e09e8",
    "id": null,
    "metadata": {},
    "name": "AppRootQuery",
    "operationKind": "query",
    "text": "query AppRootQuery(\n  $fsRef: ID\n  $focus: ID!\n  $hasFocus: Boolean!\n) {\n  rootDir\n  filesystem(ref: $fsRef) {\n    name\n    reference\n    isDir\n  }\n  neighborhood(ref: $focus) @include(if: $hasFocus) {\n    incomplete\n    focus {\n      id\n      kind\n      label\n      parentId\n    }\n    nodes {\n      id\n      kind\n      label\n      parentId\n    }\n    edges {\n      from\n      to\n      kind\n    }\n  }\n  code(ref: $focus) @include(if: $hasFocus) {\n    reference\n    language\n    nonText\n    error\n    warning\n    focusId\n    parentHref\n    segments {\n      text\n      href\n      anchorId\n      isLink\n      isDef\n      reference\n    }\n    files {\n      name\n      reference\n      isDir\n    }\n    symbols {\n      name\n      reference\n      isDir\n    }\n  }\n}\n"
  }
};
})();

(node as any).hash = "b7e009252e0fb06e60c95e8e965b6d2c";

export default node;
