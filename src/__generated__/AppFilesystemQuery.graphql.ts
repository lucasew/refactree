/**
 * @generated SignedSource<<8ff1bd52b7e694a8b3c3ff2cf84766da>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type AppFilesystemQuery$variables = {
  fsRef?: string | null | undefined;
};
export type AppFilesystemQuery$data = {
  readonly filesystem: ReadonlyArray<{
    readonly isDir: boolean;
    readonly name: string;
    readonly reference: string;
  }>;
};
export type AppFilesystemQuery = {
  response: AppFilesystemQuery$data;
  variables: AppFilesystemQuery$variables;
};

const node: ConcreteRequest = (function(){
var v0 = [
  {
    "defaultValue": null,
    "kind": "LocalArgument",
    "name": "fsRef"
  }
],
v1 = [
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
    "selections": [
      {
        "alias": null,
        "args": null,
        "kind": "ScalarField",
        "name": "name",
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "kind": "ScalarField",
        "name": "reference",
        "storageKey": null
      },
      {
        "alias": null,
        "args": null,
        "kind": "ScalarField",
        "name": "isDir",
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
    "name": "AppFilesystemQuery",
    "selections": (v1/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Operation",
    "name": "AppFilesystemQuery",
    "selections": (v1/*: any*/)
  },
  "params": {
    "cacheID": "38cc628f1b68a1dd9a4e9ce58d33be40",
    "id": null,
    "metadata": {},
    "name": "AppFilesystemQuery",
    "operationKind": "query",
    "text": "query AppFilesystemQuery(\n  $fsRef: ID\n) {\n  filesystem(ref: $fsRef) {\n    name\n    reference\n    isDir\n  }\n}\n"
  }
};
})();

(node as any).hash = "d0c1a33b098481ce0f7a29194bb82672";

export default node;
