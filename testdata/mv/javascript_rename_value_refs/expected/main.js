export function assist() {
  return 1;
}
export function use(fn) {
  return fn();
}
export const a = use(assist);
export const b = assist(1);
export const c = [assist, assist()];
export const d = { k: assist, assist };
export const e = assist;
export const f = true ? assist : null;
