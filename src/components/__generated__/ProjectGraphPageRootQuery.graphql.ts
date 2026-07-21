/**
 * @generated SignedSource<<4cf6afeb67ba904dceca2ead36770f76>>
 * @lightSyntaxTransform
 * @nogrep
 */

/* tslint:disable */
/* eslint-disable */
// @ts-nocheck

import { ConcreteRequest } from 'relay-runtime';
export type ProjectGraphPageRootQuery$variables = Record<PropertyKey, never>;
export type ProjectGraphPageRootQuery$data = {
  readonly rootDir: string;
};
export type ProjectGraphPageRootQuery = {
  response: ProjectGraphPageRootQuery$data;
  variables: ProjectGraphPageRootQuery$variables;
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
    "name": "ProjectGraphPageRootQuery",
    "selections": (v0/*: any*/),
    "type": "Query",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": [],
    "kind": "Operation",
    "name": "ProjectGraphPageRootQuery",
    "selections": (v0/*: any*/)
  },
  "params": {
    "cacheID": "4786f2d06b4d1258f962b37140cdbb0f",
    "id": null,
    "metadata": {},
    "name": "ProjectGraphPageRootQuery",
    "operationKind": "query",
    "text": "query ProjectGraphPageRootQuery {\n  rootDir\n}\n"
  }
};
})();

(node as any).hash = "2d777eefb5f24162f53704cd35db1376";

export default node;
