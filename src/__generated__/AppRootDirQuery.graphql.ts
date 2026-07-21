/**
 * @generated SignedSource<<efe634ebcab1ab5aa61cd1a5130503fb>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type AppRootDirQuery$variables = Record<PropertyKey, never>;
export type AppRootDirQuery$data = {
  readonly rootDir: string;
};
export type AppRootDirQuery = {
  response: AppRootDirQuery$data;
  variables: AppRootDirQuery$variables;
};

const node: ConcreteRequest = (function(){
var v0 = [
  {
    "alias": null,
    "args": null,
    "kind": "ScalarField",
    "name": "rootDir",
    "storageKey": null
  }
];
return {
  "fragment": {
    "argumentDefinitions": [],
    "kind": "Fragment",
    "metadata": null,
    "name": "AppRootDirQuery",
    "selections": (v0/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": [],
    "kind": "Operation",
    "name": "AppRootDirQuery",
    "selections": (v0/*: any*/)
  },
  "params": {
    "cacheID": "00ca988a21102456633fd388218c76e4",
    "id": null,
    "metadata": {},
    "name": "AppRootDirQuery",
    "operationKind": "query",
    "text": "query AppRootDirQuery {\n  rootDir\n}\n"
  }
};
})();

(node as any).hash = "b9caae0be234f0bc73d24b4883ce2303";

export default node;
