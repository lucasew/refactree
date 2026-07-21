/**
 * @generated SignedSource<<b07bf93135a0fb35ca1ba4d77480aff7>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type AppCodeQuery$variables = {
  focus: string;
};
export type AppCodeQuery$data = {
  readonly code: {
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
};
export type AppCodeQuery = {
  response: AppCodeQuery$data;
  variables: AppCodeQuery$variables;
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
  "name": "reference",
  "storageKey": null
},
v2 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "name",
    "storageKey": null
  },
  (v1/*: any*/),
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "isDir",
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
    "concreteType": "CodeDocument",
    "kind": "LinkedField",
    "name": "code",
    "plural": false,
    "selections": [
      (v1/*: any*/),
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
          (v1/*: any*/)
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
        "selections": (v2/*: any*/),
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "concreteType": "FsEntry",
        "kind": "LinkedField",
        "name": "symbols",
        "plural": true,
        "selections": (v2/*: any*/),
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
    "name": "AppCodeQuery",
    "selections": (v3/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Operation",
    "name": "AppCodeQuery",
    "selections": (v3/*: any*/)
  },
  "params": {
    "cacheID": "a1feb9a30bf8a304e73d00eb309005be",
    "id": null,
    "metadata": {},
    "name": "AppCodeQuery",
    "operationKind": "query",
    "text": "query AppCodeQuery(\n  $focus: ID!\n) {\n  code(ref: $focus) {\n    reference\n    language\n    nonText\n    error\n    warning\n    focusId\n    parentHref\n    segments {\n      text\n      href\n      anchorId\n      isLink\n      isDef\n      reference\n    }\n    files {\n      name\n      reference\n      isDir\n    }\n    symbols {\n      name\n      reference\n      isDir\n    }\n  }\n}\n"
  }
};
})();

(node as any).hash = "212a6d20e57d2b3c56a5645643b9f023";

export default node;
