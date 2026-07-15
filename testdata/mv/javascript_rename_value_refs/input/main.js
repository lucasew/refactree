export function helper() {
  return 1;
}
export function use(fn) {
  return fn();
}
export const a = use(helper);
export const b = helper(1);
export const c = [helper, helper()];
export const d = { k: helper, helper };
export const e = helper;
export const f = true ? helper : null;
